package stash_test

import (
	"sync"
	"testing"

	graphql "github.com/hasura/go-graphql-client"
	"github.com/stretchr/testify/assert"

	"github.com/smegmarip/stash-compreface-plugin/internal/stash"
)

func TestTagCache_NewTagCache(t *testing.T) {
	cache := stash.NewTagCache()
	assert.NotNil(t, cache, "cache should not be nil")
}

func TestTagCache_GetSet(t *testing.T) {
	cache := stash.NewTagCache()

	// Test cache miss
	_, found := cache.Get("test-tag")
	assert.False(t, found, "cache should be empty initially")

	// Test cache set and get
	testID := graphql.ID("123")
	cache.Set("test-tag", testID)

	retrievedID, found := cache.Get("test-tag")
	assert.True(t, found, "tag should be in cache")
	assert.Equal(t, testID, retrievedID, "retrieved ID should match")
}

func TestTagCache_MultipleTags(t *testing.T) {
	cache := stash.NewTagCache()

	// Set multiple tags
	tags := map[string]graphql.ID{
		"tag1": "1",
		"tag2": "2",
		"tag3": "3",
		"tag4": "4",
		"tag5": "5",
	}

	for name, id := range tags {
		cache.Set(name, id)
	}

	// Verify all tags can be retrieved
	for name, expectedID := range tags {
		retrievedID, found := cache.Get(name)
		assert.True(t, found, "tag %s should be in cache", name)
		assert.Equal(t, expectedID, retrievedID, "ID for tag %s should match", name)
	}
}

func TestTagCache_OverwriteValue(t *testing.T) {
	cache := stash.NewTagCache()

	// Set initial value
	cache.Set("test-tag", graphql.ID("initial"))

	// Overwrite with new value
	cache.Set("test-tag", graphql.ID("updated"))

	// Should return updated value
	retrievedID, found := cache.Get("test-tag")
	assert.True(t, found)
	assert.Equal(t, graphql.ID("updated"), retrievedID, "should return updated value")
}

func TestTagCache_ConcurrentAccess(t *testing.T) {
	cache := stash.NewTagCache()
	var wg sync.WaitGroup

	// Number of concurrent goroutines
	numGoroutines := 100
	numOperations := 100

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				tagName := "concurrent-tag"
				tagID := graphql.ID(string(rune('0' + (id % 10))))
				cache.Set(tagName, tagID)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				cache.Get("concurrent-tag")
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Verify cache is still functional
	cache.Set("final-tag", graphql.ID("999"))
	id, found := cache.Get("final-tag")
	assert.True(t, found, "cache should still work after concurrent access")
	assert.Equal(t, graphql.ID("999"), id)
}

func TestTagCache_EmptyString(t *testing.T) {
	cache := stash.NewTagCache()

	// Test empty string as key
	cache.Set("", graphql.ID("empty-key"))

	retrievedID, found := cache.Get("")
	assert.True(t, found, "empty string should be valid cache key")
	assert.Equal(t, graphql.ID("empty-key"), retrievedID)
}

func TestTagCache_LongTagName(t *testing.T) {
	cache := stash.NewTagCache()

	// Test very long tag name
	longName := "this-is-a-very-long-tag-name-with-lots-of-characters-to-test-if-the-cache-handles-long-strings-properly"
	cache.Set(longName, graphql.ID("long"))

	retrievedID, found := cache.Get(longName)
	assert.True(t, found)
	assert.Equal(t, graphql.ID("long"), retrievedID)
}

func TestTagCache_SpecialCharacters(t *testing.T) {
	cache := stash.NewTagCache()

	specialNames := []string{
		"tag with spaces",
		"tag-with-dashes",
		"tag_with_underscores",
		"tag.with.dots",
		"tag/with/slashes",
		"tag:with:colons",
		"Ταγ με ελληνικά", // Greek characters
		"标签中文",          // Chinese characters
	}

	for i, name := range specialNames {
		id := graphql.ID(string(rune('0' + i)))
		cache.Set(name, id)

		retrievedID, found := cache.Get(name)
		assert.True(t, found, "cache should handle special characters: %s", name)
		assert.Equal(t, id, retrievedID)
	}
}

// Note: Most stash package functions require GraphQL client and are tested
// in integration tests. This unit test file focuses on the TagCache which
// is pure logic without external dependencies.
