package cmd

import (
	"github.com/spf13/cobra"
)

func NewStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enable-worker-agent",
		Short: "Runs the Data Plane Worker Agent",
	}

	var (
		configPath string
	)

	cmd.Flags().StringVar(&configPath, "config", "/etc/data-plane-worker-agent/config.json", "Path to the agent configuration file")

	cmd.AddCommand(NewRunCommand(configPath))

	return cmd
}

func NewRunCommand(configPath string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Runs the worker agent with the specified configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			// This will be implemented to call the main agent logic
			return nil
		},
	}

	return cmd
}
