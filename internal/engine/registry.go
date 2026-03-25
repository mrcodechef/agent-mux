// Package engine implements the LoopEngine and adapter registry.
package engine

import (
	"fmt"

	"github.com/buildoak/agent-mux/internal/types"
)

// Registry holds registered harness adapters.
type Registry struct {
	adapters map[string]types.HarnessAdapter
	models   map[string][]string // engine name -> valid model slugs
}

// NewRegistry creates an empty adapter registry.
func NewRegistry() *Registry {
	return &Registry{
		adapters: make(map[string]types.HarnessAdapter),
		models:   make(map[string][]string),
	}
}

// Register adds a harness adapter to the registry.
func (r *Registry) Register(name string, adapter types.HarnessAdapter, validModels []string) {
	r.adapters[name] = adapter
	r.models[name] = validModels
}

// GetAdapter returns the adapter for the given engine name.
func (r *Registry) GetAdapter(name string) (types.HarnessAdapter, error) {
	adapter, ok := r.adapters[name]
	if !ok {
		engines := make([]string, 0, len(r.adapters))
		for k := range r.adapters {
			engines = append(engines, k)
		}
		return nil, fmt.Errorf("engine %q not found. Valid engines: %v", name, engines)
	}
	return adapter, nil
}

// ValidModels returns the valid model slugs for the given engine.
func (r *Registry) ValidModels(name string) []string {
	return r.models[name]
}

// EngineNames returns all registered engine names.
func (r *Registry) EngineNames() []string {
	names := make([]string, 0, len(r.adapters))
	for k := range r.adapters {
		names = append(names, k)
	}
	return names
}
