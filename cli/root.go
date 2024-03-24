// Copyright © 2024 Ken Robertson <ken@invalidlogic.com>

package cli

import (
	"os"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var (
	RootCmd = &cobra.Command{
		Use:   "chia-garden",
		Short: "A brief description of your application",
		Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
		// Uncomment the following line if your bare application
		// has an action associated with it:
		// Run: func(cmd *cobra.Command, args []string) { },
	}

	NatsUrl string
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	RootCmd.PersistentFlags().StringVarP(&NatsUrl, "nats", "n", nats.DefaultURL, "NATS connection string")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
