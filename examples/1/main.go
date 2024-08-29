package main

import (
	"context"
	"fmt"
	"github.com/doraemonkeys/ggcache"
	"time"
)

func main() {
	cache := ggcache.NewSimpleBuilder[string, int]().
		InitSize(10).
		DeleteExpiredInterval(time.Second * 5).
		Build()

	cache.Set("one", 1)
	cache.SetWithExpire("two", 2, ptrTime(time.Now().Add(time.Second*10)))

	if val, ok := cache.Get("one"); ok {
		fmt.Printf("Value for 'one': %d\n", val)
	}

	if val, expireAt, ok := cache.GetWithExpire("two"); ok {
		fmt.Printf("Value for 'two': %d, expires at: %v\n", val, expireAt)
	}

	loader := func(key string, ctx context.Context) (int, *time.Time, error) {
		value := len(key) * 10
		expireAt := ptrTime(time.Now().Add(time.Minute))
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
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
