package workers

import (
	"fmt"
	"sync"
)

var (
	registeredWorkers = make(map[string]Worker)
	mu                sync.RWMutex
)

func Register(worker Worker) error {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := registeredWorkers[worker.Name()]; exists {
		return fmt.Errorf("worker %s already registered", worker.Name())
	}

	registeredWorkers[worker.Name()] = worker
	return nil
}

func Get(name string) (Worker, bool) {
	mu.RLock()
	defer mu.RUnlock()

	worker, exists := registeredWorkers[name]
	return worker, exists
}

func List() []Worker {
	mu.RLock()
	defer mu.RUnlock()

	result := make([]Worker, 0, len(registeredWorkers))
	for _, worker := range registeredWorkers {
		result = append(result, worker)
	}
	return result
}

func GetNames() []string {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(registeredWorkers))
	for name := range registeredWorkers {
		names = append(names, name)
	}
	return names
}
