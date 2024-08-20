package ggcache

import (
	"context"
	"errors"
	"sync"
	"time"

	pq "github.com/doraemonkeys/queue/priorityQueue"
)

type (
	LoaderFunc[K comparable, V any]       func(K, context.Context) (V, error)
	LoaderExpireFunc[K comparable, V any] func(K, context.Context) (V, *time.Time, error)
	// EvictedFunc[K comparable, V any]      func(K, V)
	// PurgeVisitorFunc[K comparable, V any] func(K, V)
	// AddedFunc[K comparable, V any]        func(K, V)
	// DeserializeFunc[K comparable, V any]  func(interface{}, interface{}) (interface{}, error)
	// SerializeFunc[K comparable, V any]    func(K, V) (V, error)
)

type Cacher[K comparable, V any] interface {
	// Set inserts or updates the specified key-value pair.
	Set(key K, value V)
	// SetWithExpire inserts or updates the specified key-value pair with an expiration time.
	SetWithExpire(key K, value V, expireAt *time.Time)
	// Get returns the value for the specified key if it is present in the cache.
	Get(key K) (V, bool)
	// GetWithExpire returns the value and the expiration time for the specified key if it is present in the cache.
	GetWithExpire(key K) (V, *time.Time, bool)

	GetAndRefreshExpire(key K, expireAt *time.Time) (V, bool)

	GetOrReload(key K, loader LoaderExpireFunc[K, V], ctx context.Context) (V, error)

	// GetAll returns a slice containing all key-value pairs in the cache.
	GetAll() ([]K, []V)
	// GetAllMap returns a map containing all key-value pairs in the cache.
	GetAllMap() map[K]V

	// Remove removes the specified key from the cache if the key is present.
	// Returns true if the key was present and the key has been deleted.
	Remove(key K) (V, bool)
	// Purge removes all key-value pairs from the cache.
	Purge()
	// Keys returns a slice containing all keys in the cache.
	Keys() []K
	// Len returns the number of items in the cache.
	Len() int
	// Has returns true if the key exists in the cache.
	Has(key K) bool

	statsAccessor
}

var (
	ErrorCacheEmpty = errors.New("cache is empty")
)

type CacheType string

const (
	TYPE_SIMPLE CacheType = "simple"
	TYPE_LRU    CacheType = "lru"
	TYPE_LFU    CacheType = "lfu"
	TYPE_ARC    CacheType = "arc"
)

type BaseCache[K comparable, V any] struct {

	// loaderExpireFunc LoaderExpireFunc[K, V]

	// evictedFunc      EvictedFunc[K, V]
	// purgeVisitorFunc PurgeVisitorFunc[K, V]
	// addedFunc        AddedFunc[K, V]
	// deserializeFunc  DeserializeFunc
	// serializeFunc    SerializeFunc
	// expiration *time.Duration

	// The priority queue is used to determine the priority by expireAt.
	expireQueue   *pq.PriorityQueue[CacheItem[K, V]]
	expireQueueMu sync.Mutex
	// Removes only the expired keys from the cache.
	removeExpireKeys func([]K)
	goTickerOnce     sync.Once
	tickerDuraton    time.Duration
	size             int
	clock            Clock
	// mu            sync.RWMutex
	loadGroup *Group[K, *CacheValue[V]]
	*stats
}

func newBaseCache[K comparable, V any](size int, removeKeys func([]K)) *BaseCache[K, V] {
	return &BaseCache[K, V]{
		size:             size,
		stats:            &stats{},
		loadGroup:        NewGroup[K, *CacheValue[V]](),
		clock:            NewRealClock(),
		expireQueue:      pq.New(cacheItemExpireLessT[K, K, V, V]),
		removeExpireKeys: removeKeys,
		tickerDuraton:    time.Second * 3,
	}
}

func (c *BaseCache[K, V]) withClock(clock Clock) *BaseCache[K, V] {
	c.clock = clock
	return c
}

func (c *BaseCache[K, V]) withTickerDuration(d time.Duration) *BaseCache[K, V] {
	c.tickerDuraton = d
	return c
}

func (c *BaseCache[K, V]) pushExpire(key K, val *CacheValue[V]) {
	c.goTickerOnce.Do(func() {
		go c.removeTicker(c.tickerDuraton)
	})
	c.expireQueueMu.Lock()
	c.expireQueue.Push(CacheItem[K, V]{key, val})
	c.expireQueueMu.Unlock()
}

func (c *BaseCache[K, V]) reset() {
	c.expireQueueMu.Lock()
	c.expireQueue.Clear()
	c.expireQueueMu.Unlock()
	c.stats.reset()
}

func (c *BaseCache[K, V]) removeTicker(d time.Duration) {
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for range ticker.C {
		c.removeExpired()
	}
}

func (c *BaseCache[K, V]) removeExpired() {
	now := c.clock.Now()
	maybeExpired := make([]K, 0)
	c.expireQueueMu.Lock()
	for c.expireQueue.Len() > 0 {
		item := c.expireQueue.Top()
		// fmt.Printf("remove key: %v,expireAt: %v\n", item.key, item.expireAt)
		if item.deleted {
			c.expireQueue.Pop()
			continue
		}
		if item.IsExpired(now) {
			maybeExpired = append(maybeExpired, item.key)
			c.expireQueue.Pop()
			continue
		}
		break
	}
	c.expireQueueMu.Unlock()

	if len(maybeExpired) > 0 {
		c.removeExpireKeys(maybeExpired)
	}
}

type CacheValue[V any] struct {
	value V
	// Change expireAt will cause the inconsistency of the expiration priority queue,
	// resulting in expired keys may not be deleted in time.
	// So, change expireAt should hold the lock to avoid removing the expired keys by the removeExpired method.
	expireAt *time.Time
	deleted  bool
}

func (c *CacheValue[V]) IsExpired(now time.Time) bool {
	return c.expireAt != nil && now.After(*c.expireAt)
}

type CacheItem[K comparable, V any] struct {
	key K
	*CacheValue[V]
}

func cacheItemExpireCompare[K1 comparable, K2 comparable, V1, V2 any](a CacheItem[K1, V1], b CacheItem[K2, V2]) int {
	if a.expireAt == nil {
		if b.expireAt != nil {
			return 1
		}
		return 0
	} else if b.expireAt == nil {
		return -1
	}
	if a.expireAt.After(*b.expireAt) {
		return 1
	} else if a.expireAt.Before(*b.expireAt) {
		return -1
	} else {
		return 0
	}
}

func cacheItemExpireLessT[K1 comparable, K2 comparable, V1, V2 any](a CacheItem[K1, V1], b CacheItem[K2, V2]) bool {
	return cacheItemExpireCompare(a, b) < 0
}
