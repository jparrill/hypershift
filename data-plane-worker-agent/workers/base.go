package workers

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Task represents a worker task configuration
type Task struct {
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Enabled    bool                   `json:"enabled"`
}

// Worker interface defines the methods that each worker must implement
type Worker interface {
	// Name returns the worker name
	Name() string
	// Description returns the worker description
	Description() string
	// Reconcile implements the main reconciliation logic
	// Returns a boolean indicating if the reconciliation should be requeued soon
	Reconcile(ctx context.Context, task Task) (bool, error)
	// Validate validates the task parameters
	Validate(task Task) error
	// DefaultParameters returns the default parameters for the worker
	DefaultParameters() map[string]interface{}
}

// BaseWorker provides common functionality for workers
type BaseWorker struct {
	NameField        string
	DescriptionField string
}

func (b *BaseWorker) Name() string {
	return b.NameField
}

func (b *BaseWorker) Description() string {
	return b.DescriptionField
}

func (b *BaseWorker) DefaultParameters() map[string]interface{} {
	return make(map[string]interface{})
}

// RunWorker starts the worker reconciliation loop
func RunWorker(ctx context.Context, w Worker, task Task) error {
	logger := log.FromContext(ctx).WithValues("worker", w.Name())

	// Get check interval from parameters or use default
	checkInterval := GetStringParam(task.Parameters, "check-interval", "30s")
	interval, err := time.ParseDuration(checkInterval)
	if err != nil {
		return fmt.Errorf("invalid check interval: %w", err)
	}

	logger.Info("Starting worker reconciliation loop", "interval", interval)

	return wait.PollUntilContextCancel(ctx, interval, true, func(ctx context.Context) (bool, error) {
		// Run the reconciliation
		requeue, err := w.Reconcile(ctx, task)
		if err != nil {
			logger.Error(err, "Reconciliation failed")
			// Don't stop the loop on errors, just log them
			return false, nil
		}

		// If requeue is true, we'll run again after the interval
		// If false, we'll wait for the next interval
		return !requeue, nil
	})
}

// Helper function to get string parameter with default
func GetStringParam(params map[string]interface{}, key, defaultValue string) string {
	if val, ok := params[key].(string); ok {
		return val
	}
	return defaultValue
}
