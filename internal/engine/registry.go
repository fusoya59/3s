package engine

import (
	"fmt"
	"net/http"
)

// VQDProvider defines the interface for VQD token storage.
type VQDProvider interface {
	VQDGet(query, ua string) (string, bool)
	VQDSet(query, ua, vqd string)
}

// Registry holds available search engines.
type Registry struct {
	engines map[string]Engine
	order   []string
}

// NewRegistry creates a registry and populates it with default engines.
func NewRegistry(client *http.Client, ua string, vqd VQDProvider) *Registry {
	r := &Registry{
		engines: make(map[string]Engine),
	}
	r.register(NewBraveEngine(client, ua))
	r.register(NewDuckDuckGoEngine(client, ua, vqd))
	r.register(NewBraveNewsEngine(client, ua))
	r.register(NewBingNewsEngine(client, ua))
	return r
}

func (r *Registry) register(e Engine) {
	name := e.Name()
	r.engines[name] = e
	r.order = append(r.order, name)
}

// ByName returns an engine by name, or nil.
func (r *Registry) ByName(name string) Engine {
	return r.engines[name]
}

// All returns all registered engines.
func (r *Registry) All() []Engine {
	out := make([]Engine, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.engines[name])
	}
	return out
}

// Names returns all registered engine names.
func (r *Registry) Names() []string {
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// SelectEngines returns engines matching the given names. If names is empty, returns all.
func (r *Registry) SelectEngines(names []string) ([]Engine, error) {
	if len(names) == 0 {
		return r.All(), nil
	}

	var out []Engine
	for _, n := range names {
		e := r.ByName(n)
		if e == nil {
			return nil, fmt.Errorf("unknown engine %q (available: %v)", n, r.Names())
		}
		out = append(out, e)
	}
	return out, nil
}
