# chia-garden

chia-garden is a utility to handle the migration of plot files for the Chia
blockchain.

The difference between chia-garden and other tools out there is that most other
tools will only transfer plots from a single plotting host to a single harvester
host. If you have a larger farm, you then likely have multiple plotters pointing
to individual harvesters. This creates more overhead to manage and be efficient.

chia-garden works off of a central message bus. With this, all plotters are able
to communicate to all harvesters. Any one of your plotters can simply generate
and push out plots to any of your harvesters. chia-garden works to naturally
load balance plots as their produced to the harvesters with the fewest active
transfers and evenly distribute them across all the disks on an individual
harvester.

#### How does it handle distribution?

When a new plot file is found on a harvester, it will publish a message to NATS
notifying harvesters of the file being available and noting its size.

All the available harvesters will receive this message and generate a response
with a URL for the plotter to send the file to the harvester over HTTP.

Whichever harvester responds first to the plotter is the one which will have the
file sent to it.

Since the plotter only takes the first response, even distribution can be
naturally achieved by the harvesters responding fast or intentionally responding
slow. For instance:

* If the current harvester is already transferring a few plots, its network
  traffic is going to be pegged, so it would be less ideal to receive it. It can
  delay responding by 10ms for each active transfer it already has.
* If the current harvester's disk is getting closer to filling, it might be less
  ideal to plot compared to a harvester with a completely empty disk. So it can
  add a 5-10ms delay to when it responds.
* If the harvester has no transfers and plenty of storage, no delay, respond
  immediately!

#### Optimizations Applied

A number of optimizations are instrumented in how the transfers are performed.

1. The harvester nodes will prioritize disks with more free space. With this,
   disks will typically fill at an even rate rather than one at a time.
1. It will only allow one plot to be written to a disk at a time. This is to
   avoid fragmentation on the disk caused by concurrent plots being written.
1. Harvesters with more transfers will be deprioritized. This is to limit
   network conjestion in cases where multiple plotters are are transferring at
   the same time.

## Installation

#### Running NATS

The main prerequisite for running chia-garden is to be running the NATS message
bus. NATS is powerful, yet simple and resource efficient. If you're going big
time, you can run it [see the
docs](https://docs.nats.io/running-a-nats-service/introduction) on how to set it
up clustered. But for simplicity sake, you can simply `docker run` it.

```shell
$ docker run -p 4222:4222 -d nats:latest
```

You're now running a NATS server with port 4222 forwarding to it.

#### Running Plotters

```shell
$ docker run \
    -e GARDEN_NATS_URL=nats://1.2.3.4:4222 \
    -v /mnt/plots:/mnt/plots \
    ghcr.io/krobertson/chia-garden:dev plotter --path /mnt/plots/final
```

#### Running Harvesters

```shell
$ docker run -p 3434:3434 \
    -e GARDEN_NATS_URL=nats://1.2.3.4:4222  -e GARDEN_HARVESTER_HTTP_IP=1.2.3.4 \
    -v /mnt/plots:/mnt/plots \
    ghcr.io/krobertson/chia-garden:dev harvester --expand-path /mnt/plots
```

## Environment Variables

chia-garden can use environment variables in the place of command line
arguments. This can be a bit simpler to set values when running it within
Docker.

* `GARDEN_NATS_URL`: The nats connection string.
* `GARDEN_HARVESTER_HTTP_IP`: The IP address the `harvester` command should use to identify itself.
* `GARDEN_HARVESTER_HTTP_PORT`: The port the `harvester` command should use to
  listen for transfer connections on.
* `GARDEN_HARVESTER_MAX_TRANSFERS`: The maximum number of transfer the
  `harvester` command should allow at a time.
* `GARDEN_PLOTTER_MAX_TRANSFERS`: The maximum number of transfer the `plotter`
  command should allow at a time.
* `GARDEN_PLOTTER_SUFFIX`: The suffix to use to identify plot files. Default is
  `plot`, can be updated to `drplot` for DrPlotter.