package provider

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

var (
	ErrProviderNotFound      = errors.New("provider not found")
	ErrProviderAlreadyExists = errors.New("provider already registered")
	ErrEmptyProviderID       = errors.New("provider id cannot be empty")
)

// Registry is a thread-safe registry of webcomic Providers.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewRegistry initializes an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry. Returns an error if the ID is duplicate or empty.
func (r *Registry) Register(p Provider) error {
	if p == nil {
		return errors.New("cannot register nil provider")
	}
	id := p.ID()
	if id == "" {
		return ErrEmptyProviderID
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[id]; exists {
		return fmt.Errorf("%w: %s", ErrProviderAlreadyExists, id)
	}

	r.providers[id] = p
	return nil
}

// Get retrieves a provider by its unique ID.
func (r *Registry) Get(id string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.providers[id]
	return p, ok
}

// FindByURL searches registered providers to find the first one that matches the given URL.
func (r *Registry) FindByURL(rawURL string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, p := range r.providers {
		if p.MatchesURL(rawURL) {
			return p, true
		}
	}
	return nil, false
}

// All returns a slice of all registered providers sorted deterministically by Name.
func (r *Registry) All() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		list = append(list, p)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Name() < list[j].Name()
	})

	return list
}

// List is an alias for All.
func (r *Registry) List() []Provider {
	return r.All()
}
