package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	cmd := &cobra.Command{
		Use: "data-plane-worker-agent",
	}

	cmd.AddCommand(NewStartCommand())
	cmd.AddCommand(NewStandaloneCommand())

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func NewStandaloneCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "standalone",
		Short: "Run the worker agent in standalone mode (for testing)",
		RunE: func(cmd *cobra.Command, args []string) error {
			config := &AgentConfig{
				EnabledWorkers: []string{"sync-pullsecret"},
				WorkerConfigs: map[string]WorkerConfig{
					"sync-pullsecret": {
						Enabled: true,
						Parameters: map[string]interface{}{
							"kubelet-config-json-path": "/tmp/kubelet-config.json",
							"global-ps-secret-name":    "global-pull-secret",
							"check-interval":           "10s",
						},
					},
				},
				LogLevel: "debug",
			}

			log.Println("Starting Data Plane Worker Agent in standalone mode")
			log.Printf("Configuration: %+v", config)

			agent := NewAgent(config)
			return agent.Start()
		},
	}

	return cmd
}
