// Copyright © 2024 Ken Robertson <ken@invalidlogic.com>

package plotter

import (
	"log"
	"os"
	"path/filepath"

	"github.com/krobertson/chia-garden/cli"
	"github.com/krobertson/chia-garden/pkg/rpc"
	"github.com/krobertson/chia-garden/pkg/types"

	"github.com/fsnotify/fsnotify"
	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	maxTransfers int
	plotSuffix   string
)

func init() {
	cli.RootCmd.AddCommand(PlotterCmd)

	viper.SetDefault("plotter.max_transfers", 2)
	viper.SetDefault("plotter.suffix", "plot")

	viper.BindEnv("plotter.max_transfers")
	viper.BindEnv("plotter.suffix")

	PlotterCmd.Flags().StringSliceVarP(&plotterPaths, "path", "p", nil, "Paths to watch for plots")
	PlotterCmd.Flags().IntVarP(&maxTransfers, "max-transfers", "t", viper.GetInt("plotter.max_transfers"), "Max concurrent transfers")
	PlotterCmd.Flags().StringVarP(&plotSuffix, "suffix", "s", viper.GetString("plotter.suffix"), "The suffix or extension of plot files")

	viper.BindPFlag("plotter.max_transfers", PlotterCmd.Flags().Lookup("max-transfers"))
	viper.BindPFlag("plotter.suffix", PlotterCmd.Flags().Lookup("suffix"))
}

func cmdPlotter(cmd *cobra.Command, args []string) {
	log.Print("Starting plotter-client...")

	// connect to nats
	conn, err := nats.Connect(cli.NatsUrl, nats.MaxReconnects(-1))
	if err != nil {
		log.Fatal("Failed to connect to NATS: ", err)
	}
	defer conn.Close()

	// initialize client and create fixed worker routines
	client := rpc.NewNatsPlotterClient(conn)
	plotqueue := make(chan string, 1024)
	for i := 0; i < maxTransfers; i++ {
		go plotworker(client, plotqueue)
	}

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

				// filter to only the plot files
				if filepath.Ext(event.Name) != "."+plotSuffix {
					continue
				}

				// found new plot
				log.Printf("New plot created %s", event.Name)
				plotqueue <- event.Name

			case err, ok := <-watcher.Errors:
				if !ok {
					log.Print("Leaving watch loop")
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	// Check for existing files and add the path to the watcher
	existingFiles := make([]string, 0)
	for _, path := range plotterPaths {
		files, err := os.ReadDir(path)
		if err != nil {
			log.Fatalf("Failed to list files for path %s: %v", path, err)
		}
		for _, de := range files {
			name := de.Name()

			// filter to only plot files
			if filepath.Ext(name) != "."+plotSuffix {
				continue
			}

			existingFiles = append(existingFiles, filepath.Join(path, name))
		}

		err = watcher.Add(path)
		if err != nil {
			log.Fatal("Failed to watch plots path", err)
		}
	}

	log.Print("Ready")

	// Loop and check the existing files for plots
	for _, file := range existingFiles {
		fi, err := os.Stat(file)
		if err != nil {
			log.Printf("Failed to check info on plot %s, removing and continuing: %v", file, err)
			os.Remove(file)
			continue
		}

		// do request to see if any nodes have it
		req := &types.PlotLocateRequest{
			Name: filepath.Base(file),
			Size: uint64(fi.Size()),
		}
		resp, err := client.PlotLocate(req)

		// if a valid resp and no error, it does exist, so remove and continue
		if resp != nil && err == nil {
			log.Printf("Plot %s already exists, cleaning up", file)
			os.Remove(file)
			continue
		}

		// if resp is nil and err is a timeout error, it does not exist, send it
		if resp == nil && err == nats.ErrTimeout {
			log.Printf("Plot %s not on harvesters, queuing to send...", file)
			plotqueue <- file
			continue
		}

		// other error, log and continue
		if err != nil {
			log.Printf("Other error from NATS locate request: %v", err)
			continue
		}
	}

	// Block main goroutine forever.
	<-make(chan struct{})
}
