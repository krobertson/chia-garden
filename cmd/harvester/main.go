package main

import (
	"flag"
	"log"

	"github.com/krobertson/chia-garden/pkg/rpc"
	"github.com/krobertson/chia-garden/pkg/utils"

	"github.com/nats-io/nats.go"

	_ "net/http/pprof"
)

var (
	maxTransfers int64 = 5
)

func main() {
	var plotPaths utils.ArrayFlags

	natsUrl := flag.String("nats", nats.DefaultURL, "NATS connection string")
	flag.Var(&plotPaths, "plot", "Plots directories")
	flag.Int64Var(&maxTransfers, "max-transfers", 5, "max concurrent transfers")
	flag.Parse()

	log.Print("Starting harvester-client...")

	conn, err := nats.Connect(*natsUrl, nats.MaxReconnects(-1))
	if err != nil {
		log.Fatal("Failed to connect to NATS: ", err)
	}
	defer conn.Close()

	server, err := newHarvester(plotPaths)
	if err != nil {
		log.Fatal("Failed to initialize harvester: ", err)
	}

	// initialize the rpc
	_, err = rpc.NewNatsHarvesterListener(conn, server)
	if err != nil {
		log.Fatal("Failed to initialize NATS listener: ", err)
	}

	// Block main goroutine forever.
	log.Print("Ready")
	<-make(chan struct{})
}
