package arc

import (
	"context"
	"time"

	"github.com/doraemonkeys/ggcache"
	lru "github.com/hashicorp/golang-lru/arc/v2"
)

type Arc[K comparable, V any] struct {
	ggcache.BaseCache[K, V]
	arc *lru.ARCCache[K, *ggcache.CacheValue[V]]
	cap int
}

func (a Arc[K, V]) Set(key K, value V) {
	a.set(key, value, nil)
}

func (a Arc[K, V]) set(key K, value V, expireAt *time.Time) {
	// a.arc.
}

func (a Arc[K, V]) SetWithExpire(key K, value V, expireAt *time.Time) {
	//TODO implement me
	panic("implement me")
}

func (a Arc[K, V]) Get(key K) (V, bool) {
	//TODO implement me
	panic("implement me")
}

func (a Arc[K, V]) GetWithExpire(key K) (V, *time.Time, bool) {
	//TODO implement me
	panic("implement me")
}

func (a Arc[K, V]) GetAndRefreshExpire(key K, expireAt *time.Time) (V, bool) {
	//TODO implement me
	panic("implement me")
}

func (a Arc[K, V]) GetOrReload(key K, loader ggcache.LoaderExpireFunc[K, V], ctx context.Context) (V, error) {
	//TODO implement me
	panic("implement me")
}

func (a Arc[K, V]) GetAll() ([]K, []V) {
	//TODO implement me
	panic("implement me")
}

func (a Arc[K, V]) GetAllMap() map[K]V {
	//TODO implement me
	panic("implement me")
}

func (a Arc[K, V]) Remove(key K) (V, bool) {
	//TODO implement me
	panic("implement me")
}

func (a Arc[K, V]) Purge() {
	//TODO implement me
	panic("implement me")
}

func (a Arc[K, V]) Keys() []K {
	//TODO implement me
	panic("implement me")
}

func (a Arc[K, V]) Len() int {
	//TODO implement me
	panic("implement me")
}

func (a Arc[K, V]) Has(key K) bool {
	//TODO implement me
	panic("implement me")
}

// func NewARC[K comparable, T any](cap int) *[K, T] {
// 	arc, err := lru.NewARC[K, T](cap)
// 	if err != nil {
// 		panic(err)
// 	}
// 	return &[K, T]{arc: arc, cap: cap}
// }

// func (c *[K, T]) Cap() int {
// 	return c.cap
// }

// func (c *[K, T]) Len() int {
// 	return c.arc.Len()
// }

// func (c *[K, T]) Get(key K) (T, bool) {
// 	return c.arc.Get(key)
// }

// func (c *[K, T]) Set(key K, val T) bool {
// 	c.arc.Add(key, val)
// 	return true
// }

// func (c *[K, T]) Delete(key K) {
// 	c.arc.Remove(key)
// }

// func (c *[K, T]) IsExist(key K) bool {
// 	return c.arc.Contains(key)
// }

// func (c *[K, T]) ClearAll() {
// 	c.arc.Purge()
// }

// func (c *[K, T]) GetMulti(keys []K) map[K]T {
// 	var m = make(map[K]T, len(keys))
// 	for _, key := range keys {
// 		if val, ok := c.arc.Get(key); ok {
// 			m[key] = val
// 		}
// 	}
// 	return m
// }

// func (c *[K, T]) SetMulti(kvs map[K]T) []bool {
// 	var flags []bool
// 	for key, val := range kvs {
// 		flags = append(flags, c.Set(key, val))
// 	}
// 	return flags
// }

// func (c *[K, T]) DeleteMulti(keys []K) {
// 	for _, key := range keys {
// 		c.arc.Remove(key)
// 	}
// }
