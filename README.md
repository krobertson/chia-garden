# chia-garden

chia-garden is a utility to handle the migration of plot files for the Chia
blockchain.

It can be used to transfer plot files from dedicated plotting nodes to available
harvester nodes, optimizing for even file distribution and network conjestion.
It can work across a set of many plotters and many harvesters by communicating
over a dedicated message bus utilizing NATS.

### How does it handle distribution?

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

### Optimizations Applied

A number of optimizations are instrumented in how the transfers are performed.

1. The harvester nodes will prioritize disks with more free space. With this,
   disks will typically fill at an even rate rather than one at a time.
1. It will only allow one plot to be written to a disk at a time. This is to
   avoid fragmentation on the disk caused by concurrent plots being written.
1. Harvesters with more transfers will be deprioritized. This is to limit
   network conjestion in cases where multiple plotters are are transferring at
   the same time.