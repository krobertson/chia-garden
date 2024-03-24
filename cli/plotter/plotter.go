// Copyright © 2024 Ken Robertson <ken@invalidlogic.com>

package plotter

import (
	"log"
	"strings"

	"github.com/krobertson/chia-garden/cli"
	"github.com/krobertson/chia-garden/pkg/rpc"
	"github.com/spf13/cobra"

	"github.com/fsnotify/fsnotify"
	"github.com/nats-io/nats.go"
)

// plotterCmd represents the plotter command
var (
	PlotterCmd = &cobra.Command{
		Use:   "plotter",
		Short: "Transport new plots from plottings to harvesters",
		Long: `"chia-garden plotter" is used to monitor for newly created plot files on
plotter nodes and to find and transport them to a harvester for use.`,
		Run: cmdPlotter,
	}

	plotterPaths []string
)

func init() {
	cli.RootCmd.AddCommand(PlotterCmd)

	PlotterCmd.Flags().StringSliceVarP(&plotterPaths, "path", "p", nil, "Paths to watch for plots")
}

func cmdPlotter(cmd *cobra.Command, args []string) {
	log.Print("Starting plotter-client...")

	conn, err := nats.Connect(cli.NatsUrl, nats.MaxReconnects(-1))
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
	for _, path := range plotterPaths {
		err = watcher.Add(path)
		if err != nil {
			log.Fatal("Failed to watch plots path", err)
		}
	}

	// Block main goroutine forever.
	log.Print("Ready")
	<-make(chan struct{})
}
