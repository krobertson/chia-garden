package main

import (
	"flag"
	"log"
	"strings"

	"github.com/krobertson/chia-garden/pkg/rpc"

	"github.com/fsnotify/fsnotify"
	"github.com/nats-io/nats.go"
)

func main() {
	natsUrl := flag.String("nats", nats.DefaultURL, "NATS connection string")
	plotDir := flag.String("plot", "", "Plots directory")
	flag.Parse()

	log.Print("Starting plotter-client...")

	conn, err := nats.Connect(*natsUrl, nats.MaxReconnects(-1))
	if err != nil {
		log.Fatal("Failed to connect to NATS: ", err)
	}
	defer conn.Close()

	client := rpc.NewNatsPlotterClient(conn)

	// begin watching the plots directory
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("Failed to initialize watcher", err)
	}
	defer watcher.Close()

	// watch loop
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					log.Print("Leaving watch loop")
					return
				}

				// filter to create events
				if event.Op != fsnotify.Create {
					continue
				}

				// filter to only the *.plot files
				if !strings.HasSuffix(event.Name, ".plot") {
					continue
				}

				// found new plot
				log.Printf("New plot created %s", event.Name)
				go handlePlot(client, event.Name)

			case err, ok := <-watcher.Errors:
				if !ok {
					log.Print("Leaving watch loop")
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	// Add the plots path to the watcher
	err = watcher.Add(*plotDir)
	if err != nil {
		log.Fatal("Failed to watch plots path", err)
	}

	// Block main goroutine forever.
	log.Print("Ready")
	<-make(chan struct{})
}
