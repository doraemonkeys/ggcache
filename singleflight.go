package ggcache

// This module provides a duplicate function call suppression
// mechanism.

import (
	"context"
	"fmt"
	"sync"
)

// call is an in-flight or completed Do call
type call[V any] struct {
	// signals when the call is done
	doneCond chan struct{}
	doneOnce sync.Once
	// the value returned by the call
	val V
	// any error encountered during the call
	err error
}

func newCall[V any]() *call[V] {
	return &call[V]{
		doneCond: make(chan struct{}),
	}
}

func (c *call[V]) done(val V, err error, onDone ...func()) {
	c.doneOnce.Do(func() {
		c.val = val
		c.err = err
		close(c.doneCond)
		for _, f := range onDone {
			f()
		}
	})
}

// Group represents a class of work and forms a namespace in which
// units of work can be executed with duplicate suppression.
type Group[K comparable, V any] struct {
	mu sync.RWMutex   // protects m
	m  map[K]*call[V] // lazily initialized
}

func NewGroup[K comparable, V any]() *Group[K, V] {
	return &Group[K, V]{
		m: make(map[K]*call[V]),
	}
}

// Do executes and returns the results of the given function, making
// sure that only one execution is in-flight for a given key at a
// time. If a duplicate comes in, the duplicate caller waits for the
// original to complete and receives the same results.
//
// Please ensure that the loader function properly utilizes the context to avoid blocking operations.
func (g *Group[K, V]) Do(key K, loader LoaderFunc[K, V], ctx context.Context) (val V, loaderCalled bool, err error) {
	var caller *call[V]
	var firstCall = false

	g.mu.RLock()
	caller = g.m[key]
	g.mu.RUnlock()

	// If no in-flight call, start a new one
	if caller == nil {
		g.mu.Lock()
		if g.m[key] == nil {
			caller = newCall[V]()
			g.m[key] = caller
			firstCall = true
		} else {
			caller = g.m[key]
		}
		g.mu.Unlock()
	}

	if firstCall {
		// For performance reasons, we do not use a goroutine here
		// go g.call(caller, key, loader, ctx)
		g.call(caller, key, loader, ctx)
	} else {
		<-caller.doneCond
	}
	return caller.val, firstCall, caller.err
}

func (g *Group[K, V]) call(caller *call[V], key K, loader LoaderFunc[K, V], ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			caller.done(*new(V), fmt.Errorf("loader panic: %v, key: %v", r, key), func() { g.removeKey(key) })
		}
	}()
	val, err := loader(key, ctx)
	caller.done(val, err, func() { g.removeKey(key) })
}

func (g *Group[K, V]) removeKey(key K) {
	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()
}
