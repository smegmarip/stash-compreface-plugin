package stash

import (
	"sync"

	graphql "github.com/hasura/go-graphql-client"
)

// TagCache provides thread-safe cached tag lookups by name
type TagCache struct {
	tags map[string]graphql.ID
	mu   sync.RWMutex
}

// NewTagCache creates a new tag cache
func NewTagCache() *TagCache {
	return &TagCache{
		tags: make(map[string]graphql.ID),
	}
}

// Get retrieves a cached tag ID by name
func (tc *TagCache) Get(name string) (graphql.ID, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	id, ok := tc.tags[name]
	return id, ok
}

// Set stores a tag ID in the cache
func (tc *TagCache) Set(name string, id graphql.ID) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.tags[name] = id
}
