// Copyright © 2024 Ken Robertson <ken@invalidlogic.com>

package harvester

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/krobertson/chia-garden/cli"
	"github.com/krobertson/chia-garden/pkg/rpc"
	"github.com/krobertson/chia-garden/pkg/utils"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	_ "net/http/pprof"
)

// harvesterCmd represents the harvester command
var (
	HarvesterCmd = &cobra.Command{
		Use:   "harvester",
		Short: "Start receiving new plots on a harvester from plotters",
		Long: `"chia-garden harvester" is used to run the process of storing freshly created
plots on harvester nodes after a plotting machine has announced the
availability of a new plot file.`,
		Run: cmdHarvester,
	}

	harvesterPaths []string
	expandPaths    []string
	maxTransfers   int64
	httpServerIP   string
	httpServerPort int
)

func init() {
	cli.RootCmd.AddCommand(HarvesterCmd)

	viper.SetDefault("harvester.max_transfers", 5)
	viper.SetDefault("harvester.http_ip", utils.GetHostIP().String())
	viper.SetDefault("harvester.http_port", 3434)

	viper.BindEnv("harvester.max_transfers")
	viper.BindEnv("harvester.http_ip")
	viper.BindEnv("harvester.http_port")

	HarvesterCmd.Flags().StringSliceVarP(&harvesterPaths, "path", "p", nil, "Path to store plots")
	HarvesterCmd.Flags().StringSliceVarP(&expandPaths, "expand-path", "x", nil, "Path containing multiple directories to store plots")
	HarvesterCmd.Flags().Int64VarP(&maxTransfers, "max-transfers", "t", viper.GetInt64("harvester.max_transfers"), "Max concurrent transfers")
	HarvesterCmd.Flags().StringVarP(&httpServerIP, "http-ip", "", viper.GetString("harvester.http_ip"), "IP to use to identify itself (mainly need if in Docker)")
	HarvesterCmd.Flags().IntVarP(&httpServerPort, "http-port", "", viper.GetInt("harvester.http_port"), "Port to handle transfers")

	viper.BindPFlag("harvester.max_transfers", HarvesterCmd.Flags().Lookup("max-transfers"))
	viper.BindPFlag("harvester.http_ip", HarvesterCmd.Flags().Lookup("http-ip"))
	viper.BindPFlag("harvester.http_port", HarvesterCmd.Flags().Lookup("http-port"))
}

func cmdHarvester(cmd *cobra.Command, args []string) {
	log.Printf("GOMAXPROCS set to %d", runtime.GOMAXPROCS(0))
	log.Print("Starting harvester-client...")

	conn, err := nats.Connect(cli.NatsUrl, nats.MaxReconnects(-1))
	if err != nil {
		log.Fatal("Failed to connect to NATS: ", err)
	}
	defer conn.Close()

	// process expandPaths and append to harvesterPaths
	for _, ep := range expandPaths {
		p, err := filepath.Abs(ep)
		if err != nil {
			log.Printf("Failed to resolve path %s, skipping: %v", ep, err)
			continue
		}

		items, err := os.ReadDir(p)
		if err != nil {
			log.Fatalf("Failed to evaluate path %s, skipping: %v", p, err)
			continue
		}
		for _, de := range items {
			if !de.IsDir() {
				continue
			}

			harvesterPaths = append(harvesterPaths, filepath.Join(p, de.Name()))
		}
	}

	server, err := newHarvester(harvesterPaths)
	if err != nil {
		log.Fatal("Failed to initialize harvester: ", err)
	}

	// initialize the rpc
	_, err = rpc.NewNatsHarvesterListener(conn, server)
	if err != nil {
		log.Fatal("Failed to initialize NATS listener: ", err)
	}

	// add TERM signal handling to call to http shutdown
	shutdown := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		// close nats connection
		conn.Close()

		// http shutdown and wait for requests to finish
		err := server.httpServer.Shutdown(context.Background())
		if err != nil {
			log.Printf("HTTP server Shutdown: %v", err)
		}

		// close channel to exit
		close(shutdown)
	}()

	// Block main goroutine forever.
	log.Print("Ready")
	<-shutdown
}
