package ggcache

import (
	"context"
	"sync"
	"time"
)

// SimpleCache has no clear priority for evict cache. It depends on key-value map order.
type SimpleCache[K comparable, V any] struct {
	baseCache[K, V]
	items   map[K]*CacheValue[V]
	itemsMu sync.RWMutex
}

type SimpleCacheBuilder[K comparable, V any] struct {
	initSize              int
	deleteExpiredInterval time.Duration
	clock                 Clock
}

func NewSimpleCacheBuilder[K comparable, V any]() *SimpleCacheBuilder[K, V] {
	return &SimpleCacheBuilder[K, V]{
		initSize:              8,
		deleteExpiredInterval: time.Second * 3,
		clock:                 NewRealClock(),
	}
}

func (b *SimpleCacheBuilder[K, V]) InitSize(size int) *SimpleCacheBuilder[K, V] {
	b.initSize = size
	return b
}

func (b *SimpleCacheBuilder[K, V]) DeleteExpiredInterval(d time.Duration) *SimpleCacheBuilder[K, V] {
	b.deleteExpiredInterval = d
	return b
}

func (b *SimpleCacheBuilder[K, V]) Clock(clock Clock) *SimpleCacheBuilder[K, V] {
	b.clock = clock
	return b
}

func (b *SimpleCacheBuilder[K, V]) Build() *SimpleCache[K, V] {
	c := &SimpleCache[K, V]{}
	c.items = make(map[K]*CacheValue[V], b.initSize)
	c.baseCache = *NewBaseCache[K, V](b.initSize, c.removeBatchOnlyExpired).
		WithClock(b.clock).WithTickerDuration(b.deleteExpiredInterval)
	return c
}

func (s *SimpleCache[K, V]) Set(key K, value V) {
	s.SetWithExpire(key, value, nil)
}

func (s *SimpleCache[K, V]) SetWithExpire(key K, value V, expireAt *time.Time) {
	var val = &CacheValue[V]{value: value, expireAt: expireAt}
	s.set(key, val)
}

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
		s.baseCache.PushExpire(key, val)
	}
}

func (s *SimpleCache[K, V]) Get(key K) (V, bool) {
	v, _, ok := s.GetWithExpire(key)
	return v, ok
}

func (s *SimpleCache[K, V]) GetWithExpire(key K) (V, *time.Time, bool) {
	var item *CacheValue[V]
	s.itemsMu.RLock()
	item, ok := s.items[key]
	s.itemsMu.RUnlock()
	if !ok {
		s.IncrMissCount()
		return *new(V), nil, false
	}
	if item.IsExpired(s.clock.Now()) {
		s.IncrMissCount()
		return *new(V), nil, false
	}
	s.IncrHitCount()
	return item.value, item.expireAt, true
}

func (s *SimpleCache[K, V]) GetOrReload(key K, loader LoaderExpireFunc[K, V], ctx context.Context) (V, error) {
	var item *CacheValue[V]
	s.itemsMu.RLock()
	item, ok := s.items[key]
	s.itemsMu.RUnlock()
	if ok && !item.IsExpired(s.clock.Now()) {
		s.IncrHitCount()
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
		s.IncrMissCount()
		return *new(V), err
	}
	if loaderCalled {
		s.IncrMissCount()
	} else {
		s.IncrHitCount()
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

func (s *SimpleCache[K, V]) removeBatchOnlyExpired(keys []K) {
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

func (s *SimpleCache[K, V]) Purge() {
	s.itemsMu.Lock()
	clear(s.items)
	s.itemsMu.Unlock()

	s.baseCache.Reset()
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
