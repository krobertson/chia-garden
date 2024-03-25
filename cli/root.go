// Copyright © 2024 Ken Robertson <ken@invalidlogic.com>

package cli

import (
	"os"
	"strings"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands
var (
	RootCmd = &cobra.Command{
		Use:   "chia-garden",
		Short: "A utility to handle the transfering of plots from plotters to harvesters",
		Long: `chia-garden is a powerful utility to handle the management and transfers of
freshly created plots from plotter machines to harvester machines. It makes
it easy to manage a farm as it grows from small to large, while also
balancing storage across multiple nodes and disks.`,
	}

	NatsUrl string
)

func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	viper.SetDefault("nats_url", nats.DefaultURL)

	viper.SetEnvPrefix("garden")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.BindEnv("nats_url")

	RootCmd.PersistentFlags().StringVarP(&NatsUrl, "nats", "n", viper.GetString("nats_url"), "NATS connection string")
	RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	viper.BindPFlag("nats_url", RootCmd.Flags().Lookup("nats"))
}
