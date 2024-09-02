package main

import (
	"context"
	"fmt"
	"github.com/doraemonkeys/ggcache"
	"time"
)

func Example() {
	cache := ggcache.NewSimpleBuilder[string, int]().
		InitSize(10).
		DeleteExpiredInterval(time.Second * 5).
		Build()

	cache.Set("one", 1)
	cache.SetWithExpire("two", 2, time.Now().Add(time.Second*10))

	if val, ok := cache.Get("one"); ok {
		fmt.Printf("Value for 'one': %d\n", val)
	}

	if val, _, ok := cache.GetWithExpire("two"); ok {
		fmt.Printf("Value for 'two': %d\n", val)
	}

	loader := func(key string, ctx context.Context) (int, time.Time, error) {
		value := len(key) * 10
		expireAt := time.Now().Add(time.Minute)
		return value, expireAt, nil
	}

	ctx := context.Background()
	if val, err := cache.GetOrReload(ctx, "three", loader); err == nil {
		fmt.Printf("Value for 'three' (loaded): %d\n", val)
	}

	if val, err := cache.GetOrReload(ctx, "three", loader); err == nil {
		fmt.Printf("Value for 'three' (from cache): %d\n", val)
	}

	fmt.Printf("Cache stats: %+v\n", cache.HitRate())

	// Output:
	// Value for 'one': 1
	// Value for 'two': 2
	// Value for 'three' (loaded): 50
	// Value for 'three' (from cache): 50
	// Cache stats: 0.75
}
