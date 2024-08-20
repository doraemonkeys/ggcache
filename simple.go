package ggcache

import (
	"context"
	"sync"
	"time"
)

var _ Cacher[int, int] = &SimpleCache[int, int]{}

// SimpleCache has no clear priority for evict cache. It depends on key-value map order.
type SimpleCache[K comparable, V any] struct {
	BaseCache[K, V]
	items   map[K]*CacheValue[V]
	itemsMu sync.RWMutex
}

// SimpleCacheBuilder helps configure and build a SimpleCache.
type SimpleCacheBuilder[K comparable, V any] struct {
	initSize              int
	deleteExpiredInterval time.Duration
	clock                 Clock
}

// NewSimpleBuilder creates a new builder with default settings.
func NewSimpleBuilder[K comparable, V any]() *SimpleCacheBuilder[K, V] {
	return &SimpleCacheBuilder[K, V]{
		initSize:              8,
		deleteExpiredInterval: time.Second * 3,
		clock:                 NewRealClock(),
	}
}

// InitSize sets the initial size of the cache.
func (b *SimpleCacheBuilder[K, V]) InitSize(size int) *SimpleCacheBuilder[K, V] {
	b.initSize = size
	return b
}

// DeleteExpiredInterval sets the interval for deleting expired items.
func (b *SimpleCacheBuilder[K, V]) DeleteExpiredInterval(d time.Duration) *SimpleCacheBuilder[K, V] {
	b.deleteExpiredInterval = d
	return b
}

// Clock sets the clock used for expiration.
func (b *SimpleCacheBuilder[K, V]) Clock(clock Clock) *SimpleCacheBuilder[K, V] {
	b.clock = clock
	return b
}

// Build constructs the SimpleCache with the configured settings.
func (b *SimpleCacheBuilder[K, V]) Build() *SimpleCache[K, V] {
	c := &SimpleCache[K, V]{}
	c.items = make(map[K]*CacheValue[V], b.initSize)
	c.BaseCache = *newBaseCache[K, V](b.initSize, c.batchRemoveExpired).
		withClock(b.clock).withTickerDuration(b.deleteExpiredInterval)
	return c
}

// Set inserts or updates the specified key-value pair.
func (s *SimpleCache[K, V]) Set(key K, value V) {
	s.SetWithExpire(key, value, nil)
}

// SetWithExpire inserts or updates the specified key-value pair with an expiration time.
func (s *SimpleCache[K, V]) SetWithExpire(key K, value V, expireAt *time.Time) {
	var val = &CacheValue[V]{value: value, expireAt: expireAt}
	s.set(key, val)
}

// set adds or updates a cache entry and manages expiration.
func (s *SimpleCache[K, V]) set(key K, val *CacheValue[V]) {
	needInsertToExpireQueue := val.expireAt != nil
	s.itemsMu.Lock()
	old, exists := s.items[key]
	if exists {
		if val.expireAt == nil {
			needInsertToExpireQueue = false
		} else if old.expireAt != nil && val.expireAt.Equal(*old.expireAt) {
			needInsertToExpireQueue = false
		} else {
			needInsertToExpireQueue = true
		}
		old.value = val.value
		old.expireAt = val.expireAt
	}
	if !exists {
		s.items[key] = val
	}
	s.itemsMu.Unlock()

	if needInsertToExpireQueue {
		s.BaseCache.pushExpire(key, val)
	}
}

func (s *SimpleCache[K, V]) GetAndRefreshExpire(key K, expireAt *time.Time) (V, bool) {
	needInsertToExpireQueue := expireAt != nil
	var ok = false
	s.itemsMu.RLock()
	old, exists := s.items[key]
	if exists && !old.IsExpired(s.clock.Now()) {
		ok = true
		if expireAt == nil {
			needInsertToExpireQueue = false
		} else if old.expireAt != nil && expireAt.Equal(*old.expireAt) {
			needInsertToExpireQueue = false
		} else {
			needInsertToExpireQueue = true
		}
		old.expireAt = expireAt
	}
	s.itemsMu.RUnlock()

	if ok {
		s.incrHitCount()
	} else {
		s.incrMissCount()
		return *new(V), false
	}

	if needInsertToExpireQueue {
		s.BaseCache.pushExpire(key, old)
	}
	return old.value, true
}

// Get retrieves a value by key, returning false if not found or expired.
func (s *SimpleCache[K, V]) Get(key K) (V, bool) {
	v, _, ok := s.GetWithExpire(key)
	return v, ok
}

// GetWithExpire retrieves a value and its expiration by key.
func (s *SimpleCache[K, V]) GetWithExpire(key K) (V, *time.Time, bool) {
	var item *CacheValue[V]
	s.itemsMu.RLock()
	item, ok := s.items[key]
	s.itemsMu.RUnlock()
	if !ok {
		s.incrMissCount()
		return *new(V), nil, false
	}
	if item.IsExpired(s.clock.Now()) {
		s.incrMissCount()
		return *new(V), nil, false
	}
	s.incrHitCount()
	return item.value, item.expireAt, true
}

// GetOrReload retrieves a value by key, automatically reloading and caching it if expired or missing.
// It uses a loader function to fetch the value and leverages loadGroup to prevent redundant loads,
// improving performance by ensuring only one load operation per key at a time.
func (s *SimpleCache[K, V]) GetOrReload(key K, loader LoaderExpireFunc[K, V], ctx context.Context) (V, error) {
	var item *CacheValue[V]
	s.itemsMu.RLock()
	item, ok := s.items[key]
	s.itemsMu.RUnlock()
	if ok && !item.IsExpired(s.clock.Now()) {
		s.incrHitCount()
		return item.value, nil
	}
	var f = func(key K, ctx context.Context) (*CacheValue[V], error) {
		value, expireAt, err := loader(key, ctx)
		if err != nil {
			return nil, err
		}
		var item = &CacheValue[V]{value: value, expireAt: expireAt}
		s.set(key, item)
		return item, err
	}
	item, loaderCalled, err := s.loadGroup.Do(key, f, ctx)
	if err != nil {
		s.incrMissCount()
		return *new(V), err
	}
	if loaderCalled {
		s.incrMissCount()
	} else {
		s.incrHitCount()
	}
	return item.value, nil
}

func (s *SimpleCache[K, V]) GetAllMap() map[K]V {
	now := s.clock.Now()
	var m = make(map[K]V, len(s.items))
	s.itemsMu.RLock()
	for k, v := range s.items {
		if !v.IsExpired(now) {
			m[k] = v.value
		}
	}
	s.itemsMu.RUnlock()
	return m
}

func (s *SimpleCache[K, V]) GetAll() ([]K, []V) {
	now := s.clock.Now()
	var retK = make([]K, 0, len(s.items))
	var retV = make([]V, 0, len(s.items))
	s.itemsMu.RLock()
	for k, v := range s.items {
		if !v.IsExpired(now) {
			retK = append(retK, k)
			retV = append(retV, v.value)
		}
	}
	s.itemsMu.RUnlock()
	return retK, retV
}

// Remove deletes a key from the cache, returning the value and whether it was found.
func (s *SimpleCache[K, V]) Remove(key K) (V, bool) {
	s.itemsMu.Lock()
	defer s.itemsMu.Unlock()
	val, ok := s.items[key]
	if ok {
		delete(s.items, key)
		val.deleted = true
		return val.value, ok
	}
	return *new(V), ok
}

// RemoveBatch deletes multiple keys from the cache.
func (s *SimpleCache[K, V]) RemoveBatch(keys []K) {
	if len(keys) == 0 {
		return
	}
	s.itemsMu.Lock()
	defer s.itemsMu.Unlock()
	var val *CacheValue[V]
	var ok bool
	for _, k := range keys {
		val, ok = s.items[k]
		if ok {
			delete(s.items, k)
			val.deleted = true
		}
	}
}

// batchRemoveExpired removes only expired keys from the cache.
func (s *SimpleCache[K, V]) batchRemoveExpired(keys []K) {
	if len(keys) == 0 {
		return
	}
	now := s.clock.Now()
	s.itemsMu.Lock()
	defer s.itemsMu.Unlock()
	var val *CacheValue[V]
	var ok bool
	for _, k := range keys {
		val, ok = s.items[k]
		if ok && val.IsExpired(now) {
			delete(s.items, k)
			val.deleted = true
		}
	}
}

// Purge clears all items from the cache.
func (s *SimpleCache[K, V]) Purge() {
	s.itemsMu.Lock()
	clear(s.items)
	s.itemsMu.Unlock()

	s.BaseCache.reset()
}

func (s *SimpleCache[K, V]) Keys() []K {
	now := s.clock.Now()
	var ret = make([]K, 0, len(s.items))
	s.itemsMu.RLock()
	for k, v := range s.items {
		if !v.IsExpired(now) {
			ret = append(ret, k)
		}
	}
	s.itemsMu.RUnlock()
	return ret
}

// Len returns the number of items in the cache, including expired items that have not been cleaned up yet.
func (s *SimpleCache[K, V]) Len() int {
	return len(s.items)
}

func (s *SimpleCache[K, V]) Has(key K) bool {
	s.itemsMu.RLock()
	val, ok := s.items[key]
	s.itemsMu.RUnlock()
	if !ok {
		return false
	}
	return !val.IsExpired(s.clock.Now())
}
