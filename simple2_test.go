package ggcache_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/doraemonkeys/ggcache"
	"github.com/stretchr/testify/assert"
)

func TestSimpleCache_SetAndGet(t *testing.T) {
	cache := ggcache.NewSimpleBuilder[string, int]().Build()

	cache.Set("key1", 100)
	value, ok := cache.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, 100, value)

	_, ok = cache.Get("nonexistent")
	assert.False(t, ok)
}

func TestSimpleCache_SetWithExpire(t *testing.T) {
	cache := ggcache.NewSimpleBuilder[string, int]().Build()

	expireTime := time.Now().Add(2 * time.Second)
	cache.SetWithExpire("key1", 200, &expireTime)

	value, ok := cache.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, 200, value)

	time.Sleep(3 * time.Second) // Wait for expiration

	_, ok = cache.Get("key1")
	assert.False(t, ok)
}

func TestSimpleCache_Remove(t *testing.T) {
	cache := ggcache.NewSimpleBuilder[string, int]().Build()

	cache.Set("key1", 300)
	_, removed := cache.Remove("key1")
	assert.True(t, removed)

	_, ok := cache.Get("key1")
	assert.False(t, ok)
}

func TestSimpleCache_GetOrReload(t *testing.T) {
	cache := ggcache.NewSimpleBuilder[string, int]().Build()

	loader := func(key string, ctx context.Context) (int, *time.Time, error) {
		expireTime := time.Now().Add(2 * time.Second)
		return 400, &expireTime, nil
	}

	value, err := cache.GetOrReload(context.Background(), "key1", loader)
	assert.NoError(t, err)
	assert.Equal(t, 400, value)

	time.Sleep(3 * time.Second) // Wait for expiration

	value, err = cache.GetOrReload(context.Background(), "key1", loader)
	assert.NoError(t, err)
	assert.Equal(t, 400, value)
}

func TestSimpleCache_Purge(t *testing.T) {
	cache := ggcache.NewSimpleBuilder[string, int]().Build()

	cache.Set("key1", 500)
	cache.Set("key2", 600)

	cache.Purge()

	_, ok := cache.Get("key1")
	assert.False(t, ok)

	_, ok = cache.Get("key2")
	assert.False(t, ok)
}

func TestSimpleCache_Keys(t *testing.T) {
	cache := ggcache.NewSimpleBuilder[string, int]().Build()

	cache.Set("key1", 700)
	cache.Set("key2", 800)

	keys := cache.Keys()
	assert.ElementsMatch(t, []string{"key1", "key2"}, keys)
}

func TestSimpleCache_Len(t *testing.T) {
	cache := ggcache.NewSimpleBuilder[string, int]().Build()

	cache.Set("key1", 900)
	cache.Set("key2", 1000)

	assert.Equal(t, 2, cache.Len())

	cache.Remove("key1")
	assert.Equal(t, 1, cache.Len())
}

func TestSimpleCache_Has(t *testing.T) {
	cache := ggcache.NewSimpleBuilder[string, int]().Build()

	cache.Set("key1", 1100)
	assert.True(t, cache.Has("key1"))

	cache.Remove("key1")
	assert.False(t, cache.Has("key1"))
}

func TestSimpleCache_BasicOperations(t *testing.T) {
	cache := ggcache.NewSimpleBuilder[string, int]().Build()

	// Test Set and Get
	cache.Set("key1", 100)
	value, ok := cache.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, 100, value)

	// Test non-existent key
	value, ok = cache.Get("non-existent")
	assert.False(t, ok)
	assert.Equal(t, 0, value)

	// Test Remove
	cache.Set("key2", 200)
	_, removed := cache.Remove("key2")
	assert.True(t, removed)
	value, ok = cache.Get("key2")
	assert.False(t, ok)
	assert.Equal(t, 0, value)

	// Test Remove non-existent key
	_, removed = cache.Remove("non-existent")
	assert.False(t, removed)
}

func TestSimpleCache_Expiration(t *testing.T) {
	clock := ggcache.NewFakeClock()
	cache := ggcache.NewSimpleBuilder[string, int]().
		Clock(clock).
		DeleteExpiredInterval(time.Second).
		Build()

	// Set item with expiration
	expireAt := clock.Now().Add(time.Second * 5)
	cache.SetWithExpire("key", 100, &expireAt)

	// Check before expiration
	value, ok := cache.Get("key")
	assert.True(t, ok)
	assert.Equal(t, 100, value)

	// Check after expiration
	clock.Advance(time.Second * 6)
	value, ok = cache.Get("key")
	assert.False(t, ok)
	assert.Equal(t, 0, value)

	// Test GetWithExpire
	cache.SetWithExpire("key2", 200, &expireAt)
	_, _, ok = cache.GetWithExpire("key2")
	assert.True(t, !ok)
}

func TestSimpleCache_BatchOperations(t *testing.T) {
	cache := ggcache.NewSimpleBuilder[string, int]().Build()

	// Test SetWithExpire for multiple items
	now := time.Now()
	expireAt1 := now.Add(time.Second * 5)
	expireAt2 := now.Add(time.Second * 10)
	cache.SetWithExpire("key1", 100, &expireAt1)
	cache.SetWithExpire("key2", 200, &expireAt2)
	cache.Set("key3", 300)

	// Test GetAllMap
	allMap := cache.GetAllMap()
	assert.Equal(t, 3, len(allMap))
	assert.Equal(t, 100, allMap["key1"])
	assert.Equal(t, 200, allMap["key2"])
	assert.Equal(t, 300, allMap["key3"])

	// Test GetAll
	keys, values := cache.GetAll()
	assert.Equal(t, 3, len(keys))
	assert.Equal(t, 3, len(values))
	assert.Contains(t, keys, "key1")
	assert.Contains(t, keys, "key2")
	assert.Contains(t, keys, "key3")
	assert.Contains(t, values, 100)
	assert.Contains(t, values, 200)
	assert.Contains(t, values, 300)

	// Test RemoveBatch
	cache.RemoveBatch([]string{"key1", "key2"})
	assert.Equal(t, 1, cache.Len())
	_, ok := cache.Get("key3")
	assert.True(t, ok)
}

func TestSimpleCache_ConcurrentAccess(t *testing.T) {
	cache := ggcache.NewSimpleBuilder[int, int]().Build()
	var wg sync.WaitGroup
	numGoroutines := 100
	numOperations := 1000

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := j % 100
				cache.Set(key, j)
				_, _ = cache.Get(key)
				if j%10 == 0 {
					cache.Remove(key)
				}
			}
		}()
	}

	wg.Wait()
	assert.Less(t, cache.Len(), 100)
}

func TestSimpleCache_GetOrReload2(t *testing.T) {
	cache := ggcache.NewSimpleBuilder[string, int]().Build()

	loader := func(key string, ctx context.Context) (int, *time.Time, error) {
		value := len(key)
		expireAt := time.Now().Add(time.Second * 5)
		return value, &expireAt, nil
	}

	// Test getting a non-existent key
	value, err := cache.GetOrReload(context.Background(), "test", loader)
	assert.NoError(t, err)
	assert.Equal(t, 4, value)

	// Test getting an existing key
	value, err = cache.GetOrReload(context.Background(), "test", loader)
	assert.NoError(t, err)
	assert.Equal(t, 4, value)

	// Test with a loader that returns an error
	errorLoader := func(key string, ctx context.Context) (int, *time.Time, error) {
		return 0, nil, fmt.Errorf("loader error")
	}
	_, err = cache.GetOrReload(context.Background(), "error", errorLoader)
	assert.Error(t, err)
	assert.Equal(t, "loader error", err.Error())
}

func TestSimpleCache_AuxiliaryMethods(t *testing.T) {
	cache := ggcache.NewSimpleBuilder[string, int]().Build()

	cache.Set("key1", 100)
	cache.Set("key2", 200)
	cache.Set("key3", 300)

	// Test Keys
	keys := cache.Keys()
	assert.Equal(t, 3, len(keys))
	assert.Contains(t, keys, "key1")
	assert.Contains(t, keys, "key2")
	assert.Contains(t, keys, "key3")

	// Test Len
	assert.Equal(t, 3, cache.Len())

	// Test Has
	assert.True(t, cache.Has("key1"))
	assert.False(t, cache.Has("non-existent"))

	// Test Purge
	cache.Purge()
	assert.Equal(t, 0, cache.Len())
}
