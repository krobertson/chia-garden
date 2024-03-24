// Copyright © 2024 Ken Robertson <ken@invalidlogic.com>

package harvester

import (
	"log"

	"github.com/krobertson/chia-garden/cli"
	"github.com/krobertson/chia-garden/pkg/rpc"
	"github.com/spf13/cobra"

	"github.com/nats-io/nats.go"

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
	maxTransfers   int64
)

func init() {
	cli.RootCmd.AddCommand(HarvesterCmd)

	HarvesterCmd.Flags().StringSliceVarP(&harvesterPaths, "path", "p", nil, "Paths to store plots")
	HarvesterCmd.Flags().Int64VarP(&maxTransfers, "max-transfers", "t", 5, "Max concurrent transfers")
}

func cmdHarvester(cmd *cobra.Command, args []string) {
	log.Print("Starting harvester-client...")

	conn, err := nats.Connect(cli.NatsUrl, nats.MaxReconnects(-1))
	if err != nil {
		log.Fatal("Failed to connect to NATS: ", err)
	}
	defer conn.Close()

	server, err := newHarvester(harvesterPaths)
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
