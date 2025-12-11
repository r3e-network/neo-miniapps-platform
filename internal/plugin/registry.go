package plugin

import (
	"fmt"
	"sort"
	"sync"
)

// ServiceFactory is a function that creates a new service plugin instance.
// Each service registers its factory via init() function.
type ServiceFactory func(cfg ServiceConfig) (ServicePlugin, error)

var (
	registry = make(map[string]serviceEntry)
	mu       sync.RWMutex
)

// serviceEntry holds a factory and its metadata.
type serviceEntry struct {
	factory ServiceFactory
	info    ServiceInfo
}

// Register adds a service factory to the registry.
// This should be called in each service's init() function.
// Panics if a service with the same ID is already registered.
func Register(id string, info ServiceInfo, factory ServiceFactory) {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := registry[id]; exists {
		panic(fmt.Sprintf("plugin: service %q already registered", id))
	}

	info.ID = id // Ensure ID is set
	registry[id] = serviceEntry{
		factory: factory,
		info:    info,
	}
}

// Get returns a service factory by ID.
// Returns false if the service is not registered.
func Get(id string) (ServiceFactory, bool) {
	mu.RLock()
	defer mu.RUnlock()

	entry, ok := registry[id]
	if !ok {
		return nil, false
	}
	return entry.factory, true
}

// MustGet returns a service factory by ID or panics if not found.
func MustGet(id string) ServiceFactory {
	factory, ok := Get(id)
	if !ok {
		panic(fmt.Sprintf("plugin: service %q not registered. Available: %v", id, List()))
	}
	return factory
}

// List returns all registered service IDs in sorted order.
func List() []string {
	mu.RLock()
	defer mu.RUnlock()

	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Info returns the ServiceInfo for a registered service.
// Returns false if the service is not registered.
func Info(id string) (ServiceInfo, bool) {
	mu.RLock()
	defer mu.RUnlock()

	entry, ok := registry[id]
	if !ok {
		return ServiceInfo{}, false
	}
	return entry.info, true
}

// AllInfo returns ServiceInfo for all registered services.
func AllInfo() []ServiceInfo {
	mu.RLock()
	defer mu.RUnlock()

	infos := make([]ServiceInfo, 0, len(registry))
	for _, entry := range registry {
		infos = append(infos, entry.info)
	}
	return infos
}

// Count returns the number of registered services.
func Count() int {
	mu.RLock()
	defer mu.RUnlock()
	return len(registry)
}

// IsRegistered checks if a service is registered.
func IsRegistered(id string) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := registry[id]
	return ok
}
