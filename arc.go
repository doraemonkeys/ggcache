package ggcache

import (
	lru "github.com/hashicorp/golang-lru/arc/v2"
)

type myARC[K comparable, T any] struct {
	arc *lru.ARCCache[K, T]
	cap int
}

func NewARC[K comparable, T any](cap int) *myARC[K, T] {
	arc, err := lru.NewARC[K, T](cap)
	if err != nil {
		panic(err)
	}
	return &myARC[K, T]{arc: arc, cap: cap}
}

func (c *myARC[K, T]) Cap() int {
	return c.cap
}

func (c *myARC[K, T]) Len() int {
	return c.arc.Len()
}

func (c *myARC[K, T]) Get(key K) (T, bool) {
	return c.arc.Get(key)
}

func (c *myARC[K, T]) Set(key K, val T) bool {
	c.arc.Add(key, val)
	return true
}

func (c *myARC[K, T]) Delete(key K) {
	c.arc.Remove(key)
}

func (c *myARC[K, T]) IsExist(key K) bool {
	return c.arc.Contains(key)
}

func (c *myARC[K, T]) ClearAll() {
	c.arc.Purge()
}

func (c *myARC[K, T]) GetMulti(keys []K) map[K]T {
	var m = make(map[K]T, len(keys))
	for _, key := range keys {
		if val, ok := c.arc.Get(key); ok {
			m[key] = val
		}
	}
	return m
}

func (c *myARC[K, T]) SetMulti(kvs map[K]T) []bool {
	var flags []bool
	for key, val := range kvs {
		flags = append(flags, c.Set(key, val))
	}
	return flags
}

func (c *myARC[K, T]) DeleteMulti(keys []K) {
	for _, key := range keys {
		c.arc.Remove(key)
	}
}
