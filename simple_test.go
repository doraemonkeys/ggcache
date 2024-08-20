package ggcache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"math/rand/v2"

	"github.com/stretchr/testify/assert"
)

func TestSimpleCache(t *testing.T) {
	// 创建一个新的 SimpleCache
	cache := NewSimpleBuilder[string, int]().
		InitSize(10).
		DeleteExpiredInterval(time.Second).
		Build()

	// 测试 Set 和 Get
	cache.Set("key1", 100)
	value, ok := cache.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, 100, value)

	// 测试不存在的 key
	_, ok = cache.Get("non-existent")
	assert.False(t, ok)

	// 测试 SetWithExpire 和 GetWithExpire
	expireTime := time.Now().Add(50 * time.Millisecond)
	cache.SetWithExpire("key2", 200, &expireTime)

	value, expiryTime, ok := cache.GetWithExpire("key2")
	assert.True(t, ok)
	assert.Equal(t, 200, value)
	assert.Equal(t, expireTime, *expiryTime)
	_, _, ok = cache.GetWithExpire("key_non_existent")
	assert.False(t, ok)

	// 等待过期
	time.Sleep(100 * time.Millisecond)
	_, ok = cache.Get("key2")
	assert.False(t, ok)

	// 测试 Remove
	cache.Set("key3", 300)
	_, ok = cache.Remove("key3")
	assert.True(t, ok)
	_, ok = cache.Get("key3")
	assert.False(t, ok)

	// 测试 GetOrReload
	loader := func(key string, ctx context.Context) (int, *time.Time, error) {
		return 400, nil, nil
	}
	value, err := cache.GetOrReload("key4", loader, context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 400, value)

	// 测试 Purge
	cache.Purge()
	assert.Equal(t, 0, cache.Len())

	// 测试 RemoveBatch empty
	cache.itemsMu.Lock()
	cache.RemoveBatch([]string{})
	cache.batchRemoveExpired([]string{})
	cache.itemsMu.Unlock()
}

func TestSimpleCacheConcurrent(t *testing.T) {
	t.Run("Concurrent Set and Get", func(t *testing.T) {
		cache := NewSimpleBuilder[int, int]().Build()
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				cache.Set(i, i*2)
				v, ok := cache.Get(i)
				if !ok || v != i*2 {
					t.Errorf("Expected %d, got %d, exists: %v", i*2, v, ok)
				}
			}(i)
		}
		wg.Wait()
	})

	t.Run("Concurrent Set with Expire and Get", func(t *testing.T) {
		cache := NewSimpleBuilder[int, int]().Build()
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				expireAt := time.Now().Add(time.Millisecond * time.Duration(rand.IntN(100)))
				cache.SetWithExpire(i, i*2, &expireAt)
				time.Sleep(time.Millisecond * time.Duration(rand.IntN(150)))
				_, _ = cache.Get(i)
				// We don't check the value here because it may or may not have expired
				// Item might have expired, which is fine
			}(i)
		}
		wg.Wait()
	})

	t.Run("Concurrent Get or Reload", func(t *testing.T) {
		cache := NewSimpleBuilder[int, int]().Build()
		var wg sync.WaitGroup
		loader := func(key int, ctx context.Context) (int, *time.Time, error) {
			return key * 3, nil, nil
		}
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				v, err := cache.GetOrReload(i, loader, context.Background())
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if v != i*3 {
					t.Errorf("Expected %d, got %d", i*3, v)
				}
			}(i)
		}
		wg.Wait()
	})

	t.Run("Concurrent Set and Remove", func(t *testing.T) {
		cache := NewSimpleBuilder[int, int]().Build()
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(2)
			go func(i int) {
				defer wg.Done()
				cache.Set(i, i)
			}(i)
			go func(i int) {
				defer wg.Done()
				cache.Remove(i)
			}(i)
		}
		wg.Wait()
		// After this, some keys might exist and some might not, which is expected
	})

	t.Run("Concurrent GetAll and Set", func(t *testing.T) {
		cache := NewSimpleBuilder[int, int]().Build()
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(2)
			go func(i int) {
				defer wg.Done()
				cache.Set(i, i)
			}(i)
			go func() {
				defer wg.Done()
				cache.GetAllMap()
			}()
		}
		wg.Wait()
	})
}

func TestSimpleCacheConcurrentSetAndGet(t *testing.T) {
	cache := NewSimpleBuilder[string, int]().Build()
	const goroutines = 1000
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", i)
			cache.Set(key, i)
			value, ok := cache.Get(key)
			if !ok || value != i {
				t.Errorf("Expected (%d, true) for key %s, got (%d, %v)", i, key, value, ok)
			}
		}(i)
	}

	wg.Wait()
}

func TestSimpleCacheConcurrentSetWithExpireAndGet(t *testing.T) {
	cache := NewSimpleBuilder[string, int]().Build()
	const goroutines = 1000
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", i)
			expireAt := time.Now().Add(time.Hour)
			cache.SetWithExpire(key, i, &expireAt)
			value, exp, ok := cache.GetWithExpire(key)
			if !ok || value != i || exp == nil || !exp.Equal(expireAt) {
				t.Errorf("Unexpected result for key %s: (%d, %v, %v)", key, value, exp, ok)
			}
		}(i)
	}

	wg.Wait()
}

func TestSimpleCacheConcurrentGetOrReload(t *testing.T) {
	cache := NewSimpleBuilder[string, int]().Build()
	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	loader := func(key string, ctx context.Context) (int, *time.Time, error) {
		value := len(key)
		return value, nil, nil
	}

	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", i)
			value, err := cache.GetOrReload(key, loader, context.Background())
			if err != nil || value != len(key) {
				t.Errorf("Unexpected result for key %s: (%d, %v)", key, value, err)
			}
		}(i)
	}

	wg.Wait()
}

func TestSimpleCacheConcurrentRemove(t *testing.T) {
	cache := NewSimpleBuilder[string, int]().Build()
	const goroutines = 1000
	var wg sync.WaitGroup
	wg.Add(goroutines)

	// First, set all the values
	for i := 0; i < goroutines; i++ {
		key := fmt.Sprintf("key%d", i)
		cache.Set(key, i)
	}

	// Now concurrently remove them
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", i)
			_, removed := cache.Remove(key)
			if !removed {
				t.Errorf("Failed to remove key %s", key)
			}
			_, ok := cache.Get(key)
			if ok {
				t.Errorf("Key %s still exists after removal", key)
			}
		}(i)
	}

	wg.Wait()
}

func TestSimpleCacheConcurrentGetAllAndPurge(t *testing.T) {
	cache := NewSimpleBuilder[string, int]().Build()
	const goroutines = 100
	var wg sync.WaitGroup

	// Set initial values
	for i := 0; i < goroutines; i++ {
		cache.Set(fmt.Sprintf("key%d", i), i)
	}

	// Concurrently get all and purge
	wg.Add(1)
	go func() {
		defer wg.Done()
		cache.Purge()
	}()

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			keys, values := cache.GetAll()
			if len(keys) > goroutines || len(values) > goroutines {
				t.Errorf("Unexpected number of items: keys=%d, values=%d", len(keys), len(values))
			}
		}()
	}

	wg.Wait()

	// Verify cache is empty after purge
	if cache.Len() != 0 {
		t.Errorf("Cache not empty after purge, length: %d", cache.Len())
	}
}

func TestSimpleCache_ConcurrentReadWrite(t *testing.T) {
	cache := NewSimpleBuilder[string, int]().
		InitSize(1000).
		DeleteExpiredInterval(time.Second).
		Build()

	const (
		numGoroutines = 100
		numOperations = 10000
		testDuration  = 15 * time.Second
	)

	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations && time.Since(start) < testDuration; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				value := rand.IntN(1000)

				if rand.Float32() < 0.7 { // 70% reads, 30% writes
					_, _ = cache.Get(key)
				} else {
					cache.Set(key, value)
				}
			}
		}(i)
	}

	wg.Wait()
	t.Logf("Concurrent read/write test completed in %v", time.Since(start))
}

func TestSimpleCache_ConcurrentExpiration(t *testing.T) {
	cache := NewSimpleBuilder[string, int]().
		InitSize(10000).
		DeleteExpiredInterval(time.Second).
		Build()

	const (
		numItems        = 100000
		expirationRange = 10 * time.Second
		testDuration    = 13 * time.Second
	)

	// Populate cache with items that will expire at different times
	for i := 0; i < numItems; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := rand.IntN(1000)
		expireAt := time.Now().Add(time.Duration(rand.Int64N(int64(expirationRange))))
		cache.SetWithExpire(key, value, &expireAt)
	}

	start := time.Now()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for time.Since(start) < testDuration {
		<-ticker.C
		t.Logf("Current cache size: %d", cache.Len())
		if cache.Len() == 0 {
			break
		}
	}

	t.Logf("Final cache size after expiration test: %d", cache.Len())
}

func TestSimpleCache_ConcurrentLoadAndCache(t *testing.T) {
	cache := NewSimpleBuilder[string, int]().
		InitSize(1000).
		DeleteExpiredInterval(time.Second).
		Build()

	const (
		numGoroutines = 50
		numOperations = 5000
		testDuration  = 10 * time.Second
	)

	loader := func(key string, ctx context.Context) (int, *time.Time, error) {
		time.Sleep(10 * time.Millisecond) // Simulate some work
		value := rand.IntN(1000)
		expireAt := time.Now().Add(5 * time.Second)
		return value, &expireAt, nil
	}

	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations && time.Since(start) < testDuration; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j%100) // Reuse some keys to test caching
				_, err := cache.GetOrReload(key, loader, context.Background())
				if err != nil {
					t.Errorf("Error loading key %s: %v", key, err)
				}
			}
		}(i)
	}

	wg.Wait()
	t.Logf("Concurrent load and cache test completed in %v", time.Since(start))
}

func TestSimpleCache_Set(t *testing.T) {
	clock := NewFakeClock()
	cache := NewSimpleBuilder[string, int]().Clock(clock).Build()

	t.Run("Set new key-value pair", func(t *testing.T) {
		cache.Set("key1", 100)
		value, ok := cache.Get("key1")
		assert.True(t, ok)
		assert.Equal(t, 100, value)
	})

	t.Run("Update existing key", func(t *testing.T) {
		cache.Set("key1", 200)
		value, ok := cache.Get("key1")
		assert.True(t, ok)
		assert.Equal(t, 200, value)
	})

	t.Run("Set with expiration", func(t *testing.T) {
		expireAt := clock.Now().Add(time.Hour)
		cache.SetWithExpire("key2", 300, &expireAt)
		value, expire, ok := cache.GetWithExpire("key2")
		assert.True(t, ok)
		assert.Equal(t, 300, value)
		assert.Equal(t, expireAt, *expire)
	})

	t.Run("Update existing key with new expiration", func(t *testing.T) {
		expireAt := clock.Now().Add(2 * time.Hour)
		cache.SetWithExpire("key2", 400, &expireAt)
		value, expire, ok := cache.GetWithExpire("key2")
		assert.True(t, ok)
		assert.Equal(t, 400, value)
		assert.Equal(t, expireAt, *expire)
	})

	t.Run("Remove expiration from existing key", func(t *testing.T) {
		cache.Set("key2", 500)
		value, expire, ok := cache.GetWithExpire("key2")
		assert.True(t, ok)
		assert.Equal(t, 500, value)
		assert.Nil(t, expire)
	})

	t.Run("Expired key handling", func(t *testing.T) {
		expireAt := clock.Now().Add(time.Minute)
		cache.SetWithExpire("key3", 600, &expireAt)

		// Verify key exists before expiration
		value, ok := cache.Get("key3")
		assert.True(t, ok)
		assert.Equal(t, 600, value)

		// Advance clock past expiration time
		clock.Advance(2 * time.Minute)

		// Verify key no longer accessible
		_, ok = cache.Get("key3")
		assert.False(t, ok)
	})

	t.Run("Set after expiration", func(t *testing.T) {
		expireAt := clock.Now().Add(-time.Minute) // Expired 1 minute ago
		cache.SetWithExpire("key4", 700, &expireAt)

		// Verify key is not accessible due to immediate expiration
		_, ok := cache.Get("key4")
		assert.False(t, ok)

		// Set new value for the same key
		cache.Set("key4", 800)

		// Verify new value is accessible
		value, ok := cache.Get("key4")
		assert.True(t, ok)
		assert.Equal(t, 800, value)
	})

	t.Run("Set with same expiration time", func(t *testing.T) {
		clock := NewFakeClock()
		cache := NewSimpleBuilder[string, int]().Clock(clock).Build()

		expireAt := clock.Now().Add(time.Hour)

		// First set
		cache.SetWithExpire("key5", 900, &expireAt)

		// Get the initial value and verify
		value, expire, ok := cache.GetWithExpire("key5")
		assert.True(t, ok)
		assert.Equal(t, 900, value)
		assert.Equal(t, expireAt, *expire)

		// Set again with the same expiration time
		cache.SetWithExpire("key5", 1000, &expireAt)

		// Verify the value is updated but expiration time remains the same
		value, newExpire, ok := cache.GetWithExpire("key5")
		assert.True(t, ok)
		assert.Equal(t, 1000, value)
		assert.Equal(t, expireAt, *newExpire)

		// Verify that the expiration queue wasn't unnecessarily modified
		// This is an internal implementation detail, so we need to be careful about testing it
		// You might need to add a method to expose the queue length for testing purposes
		// assert.Equal(t, 1, cache.baseCache.expireQueue.Len())

		// Advance time to just before expiration
		clock.Advance(59 * time.Minute)
		value, ok = cache.Get("key5")
		assert.True(t, ok)
		assert.Equal(t, 1000, value)

		// Advance time to after expiration
		clock.Advance(2 * time.Minute)
		_, ok = cache.Get("key5")
		assert.False(t, ok)
	})
}

func TestSimpleCache2(t *testing.T) {
	t.Run("Set and Get", func(t *testing.T) {
		cache := NewSimpleBuilder[string, int]().Build()
		cache.Set("key1", 100)

		value, ok := cache.Get("key1")
		assert.True(t, ok)
		assert.Equal(t, 100, value)

		_, ok = cache.Get("non-existent")
		assert.False(t, ok)
	})

	t.Run("SetWithExpire and GetWithExpire", func(t *testing.T) {
		cache := NewSimpleBuilder[string, int]().Build()
		expireAt := time.Now().Add(time.Hour)
		cache.SetWithExpire("key1", 100, &expireAt)

		value, expire, ok := cache.GetWithExpire("key1")
		assert.True(t, ok)
		assert.Equal(t, 100, value)
		assert.Equal(t, expireAt.Unix(), expire.Unix())
	})

	t.Run("GetOrReload", func(t *testing.T) {
		cache := NewSimpleBuilder[string, int]().Build()
		loader := func(key string, ctx context.Context) (int, *time.Time, error) {
			return 200, nil, nil
		}

		value, err := cache.GetOrReload("key1", loader, context.Background())
		assert.NoError(t, err)
		assert.Equal(t, 200, value)

		// Check if the value is cached
		cachedValue, ok := cache.Get("key1")
		assert.True(t, ok)
		assert.Equal(t, 200, cachedValue)
	})

	t.Run("Remove", func(t *testing.T) {
		cache := NewSimpleBuilder[string, int]().Build()
		cache.Set("key1", 100)

		value, ok := cache.Remove("key1")
		assert.True(t, ok)
		assert.Equal(t, 100, value)

		_, ok = cache.Get("key1")
		assert.False(t, ok)
	})

	t.Run("Purge", func(t *testing.T) {
		cache := NewSimpleBuilder[string, int]().Build()
		cache.Set("key1", 100)
		cache.Set("key2", 200)

		cache.Purge()

		assert.Equal(t, 0, cache.Len())
	})

	t.Run("Expiration", func(t *testing.T) {
		mockClock := NewFakeClock()
		cache := NewSimpleBuilder[string, int]().
			Clock(mockClock).
			DeleteExpiredInterval(time.Second).
			Build()

		expireAt := mockClock.Now().Add(time.Hour)
		cache.SetWithExpire("key1", 100, &expireAt)

		// Advance time beyond expiration
		mockClock.Advance(2 * time.Hour)

		_, ok := cache.Get("key1")
		assert.False(t, ok)
	})
}

func TestSimpleCache_RemoveBeforeExpiration(t *testing.T) {
	mockClock := NewFakeClock()
	deleteExpiredInterval := 3 * time.Second
	cache := NewSimpleBuilder[string, int]().
		Clock(mockClock).
		DeleteExpiredInterval(deleteExpiredInterval).
		Build()

	// 设置一个10秒后过期的键值对
	expireAt := mockClock.Now().Add(3 * time.Second)
	cache.SetWithExpire("key1", 100, &expireAt)

	// 确保键值对被正确设置
	value, ok := cache.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, 100, value)

	// 在过期之前删除键
	_, ok = cache.Remove("key1")
	assert.True(t, ok)

	// 确保键已被删除
	_, ok = cache.Get("key1")
	assert.False(t, ok)

	// 记录当前缓存长度
	initialLen := cache.Len()

	// 推进时间超过过期时间和清除间隔
	mockClock.Advance(4 * time.Second)       // 过了过期时间
	mockClock.Advance(deleteExpiredInterval) // 过了清除间隔

	// 等待一小段时间，确保清除过程有机会运行
	time.Sleep(50 * time.Millisecond)

	// 检查缓存长度是否保持不变
	assert.Equal(t, initialLen, cache.Len(), "Cache length should not change after expiration of a removed key")

	// 再次尝试获取键，确保它仍然不存在
	_, ok = cache.Get("key1")
	assert.False(t, ok, "Removed key should not reappear after expiration")
}

func TestSimpleCache3(t *testing.T) {
	cache := NewSimpleBuilder[string, int]().Build()

	t.Run("Set and Get", func(t *testing.T) {
		cache.Set("key1", 100)
		value, ok := cache.Get("key1")
		assert.True(t, ok)
		assert.Equal(t, 100, value)

		_, ok = cache.Get("non-existent")
		assert.False(t, ok)
	})

	t.Run("SetWithExpire and GetWithExpire", func(t *testing.T) {
		expireAt := time.Now().Add(time.Hour)
		cache.SetWithExpire("key2", 200, &expireAt)

		value, expirationTime, ok := cache.GetWithExpire("key2")
		assert.True(t, ok)
		assert.Equal(t, 200, value)
		assert.Equal(t, &expireAt, expirationTime)
	})

	t.Run("Remove", func(t *testing.T) {
		cache.Set("key3", 300)
		value, ok := cache.Remove("key3")
		assert.True(t, ok)
		assert.Equal(t, 300, value)

		_, ok = cache.Get("key3")
		assert.False(t, ok)
	})

	t.Run("RemoveBatch", func(t *testing.T) {
		cache.Set("key4", 400)
		cache.Set("key5", 500)
		cache.RemoveBatch([]string{"key4", "key5", "non-existent"})

		_, ok := cache.Get("key4")
		assert.False(t, ok)
		_, ok = cache.Get("key5")
		assert.False(t, ok)
	})

	t.Run("GetOrReload", func(t *testing.T) {
		loader := func(key string, ctx context.Context) (int, *time.Time, error) {
			return 600, nil, nil
		}

		value, err := cache.GetOrReload("key6", loader, context.Background())
		assert.NoError(t, err)
		assert.Equal(t, 600, value)

		// Second call should return cached value
		value, err = cache.GetOrReload("key6", loader, context.Background())
		assert.NoError(t, err)
		assert.Equal(t, 600, value)
	})

	t.Run("GetAllMap and GetAll", func(t *testing.T) {
		cache.Purge()
		cache.Set("key7", 700)
		cache.Set("key8", 800)

		allMap := cache.GetAllMap()
		assert.Equal(t, 2, len(allMap))
		assert.Equal(t, 700, allMap["key7"])
		assert.Equal(t, 800, allMap["key8"])

		keys, values := cache.GetAll()
		assert.Equal(t, 2, len(keys))
		assert.Equal(t, 2, len(values))
		assert.Contains(t, keys, "key7")
		assert.Contains(t, keys, "key8")
		assert.Contains(t, values, 700)
		assert.Contains(t, values, 800)
	})

	t.Run("Expiration", func(t *testing.T) {
		mockClock := NewFakeClock()
		cache = NewSimpleBuilder[string, int]().Clock(mockClock).Build()

		expireAt := mockClock.Now().Add(time.Hour)
		cache.SetWithExpire("key9", 900, &expireAt)

		// Before expiration
		value, ok := cache.Get("key9")
		assert.True(t, ok)
		assert.Equal(t, 900, value)

		// After expiration
		mockClock.Advance(2 * time.Hour)
		_, ok = cache.Get("key9")
		assert.False(t, ok)
	})

	t.Run("Len and Has", func(t *testing.T) {
		cache.Purge()
		cache.Set("key10", 1000)
		cache.Set("key11", 1100)

		assert.Equal(t, 2, cache.Len())
		assert.True(t, cache.Has("key10"))
		assert.True(t, cache.Has("key11"))
		assert.False(t, cache.Has("non-existent"))
	})

	t.Run("Keys", func(t *testing.T) {
		cache.Purge()
		cache.Set("key12", 1200)
		cache.Set("key13", 1300)

		keys := cache.Keys()
		assert.Equal(t, 2, len(keys))
		assert.Contains(t, keys, "key12")
		assert.Contains(t, keys, "key13")
	})

	t.Run("Mixed operations", func(t *testing.T) {
		cache.Purge()
		cache.Set("key14", 1400)
		cache.Set("key15", 1500)
		cache.Set("key16", 1600)

		// Remove a key
		cache.Remove("key15")

		// Add a new key
		cache.Set("key17", 1700)

		// Check remaining keys
		assert.True(t, cache.Has("key14"))
		assert.False(t, cache.Has("key15"))
		assert.True(t, cache.Has("key16"))
		assert.True(t, cache.Has("key17"))

		// Verify values
		value, ok := cache.Get("key14")
		assert.True(t, ok)
		assert.Equal(t, 1400, value)

		value, ok = cache.Get("key16")
		assert.True(t, ok)
		assert.Equal(t, 1600, value)

		value, ok = cache.Get("key17")
		assert.True(t, ok)
		assert.Equal(t, 1700, value)

		// Final length check
		assert.Equal(t, 3, cache.Len())
	})

	t.Run("GetOrReload with multiple calls", func(t *testing.T) {
		cache := NewSimpleBuilder[string, int]().Build()
		loadCount := 0
		loader := func(key string, ctx context.Context) (int, *time.Time, error) {
			loadCount++
			time.Sleep(200 * time.Millisecond)
			return 100, nil, nil
		}

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				value, err := cache.GetOrReload("key18", loader, context.Background())
				assert.NoError(t, err)
				assert.Equal(t, 100, value)
			}()
		}

		wg.Wait()

		// Ensure loader was only called once
		assert.Equal(t, 1, loadCount)

		// Verify value
		value, ok := cache.Get("key18")
		assert.True(t, ok)
		assert.Equal(t, 100, value)
	})

}

func TestSimpleCache_GetRefreshExpire(t *testing.T) {
	// 创建一个模拟时钟，方便控制时间
	mockClock := NewFakeClock()

	// 使用 SimpleCacheBuilder 创建缓存实例
	cache := NewSimpleBuilder[string, int]().
		Clock(mockClock).
		Build()

	// 测试用例1: 缓存未命中
	t.Run("Cache Miss", func(t *testing.T) {
		value, ok := cache.GetAndRefreshExpire("key1", nil)
		assert.False(t, ok)
		assert.Equal(t, 0, value)
		assert.Equal(t, uint64(1), cache.MissCount())
	})

	// 测试用例2: 设置值后缓存命中
	t.Run("Cache Hit After Set", func(t *testing.T) {
		cache.Set("key2", 100)
		value, ok := cache.GetAndRefreshExpire("key2", nil)
		assert.True(t, ok)
		assert.Equal(t, 100, value)
		assert.Equal(t, uint64(1), cache.HitCount())
	})

	// 测试用例3: 刷新过期时间
	t.Run("Refresh Expire Time", func(t *testing.T) {
		now := mockClock.Now()
		expireAt := now.Add(time.Hour)
		cache.SetWithExpire("key3", 200, &expireAt)

		// 刷新过期时间
		newExpireAt := now.Add(2 * time.Hour)
		value, ok := cache.GetAndRefreshExpire("key3", &newExpireAt)
		assert.True(t, ok)
		assert.Equal(t, 200, value)

		// 验证过期时间已更新
		_, expireTime, exists := cache.GetWithExpire("key3")
		assert.True(t, exists)
		assert.Equal(t, newExpireAt, *expireTime)
	})

	// 测试用例4: 移除过期时间
	t.Run("Remove Expire Time", func(t *testing.T) {
		now := mockClock.Now()
		expireAt := now.Add(time.Hour)
		cache.SetWithExpire("key4", 300, &expireAt)

		// 移除过期时间
		value, ok := cache.GetAndRefreshExpire("key4", nil)
		assert.True(t, ok)
		assert.Equal(t, 300, value)

		// 验证过期时间已被移除
		_, expireTime, exists := cache.GetWithExpire("key4")
		assert.True(t, exists)
		assert.Nil(t, expireTime)
	})

	// 测试用例5: 过期项的处理
	t.Run("Expired Item", func(t *testing.T) {
		now := mockClock.Now()
		expireAt := now.Add(time.Hour)
		cache.SetWithExpire("key5", 400, &expireAt)

		// 模拟时间流逝，使项目过期
		mockClock.Advance(2 * time.Hour)

		// 尝试获取过期项
		value, ok := cache.GetAndRefreshExpire("key5", nil)
		assert.False(t, ok)
		assert.Equal(t, 0, value)
		assert.Equal(t, uint64(2), cache.MissCount())
	})

	// 测试用例6: 刷新相同的过期时间
	t.Run("Refresh With Same Expire Time", func(t *testing.T) {
		now := mockClock.Now()
		expireAt := now.Add(time.Hour)
		cache.SetWithExpire("key6", 500, &expireAt)

		// 使用相同的过期时间刷新
		value, ok := cache.GetAndRefreshExpire("key6", &expireAt)
		assert.True(t, ok)
		assert.Equal(t, 500, value)

		// 验证过期时间未变
		_, actualExpireTime, exists := cache.GetWithExpire("key6")
		assert.True(t, exists)
		assert.Equal(t, expireAt, *actualExpireTime)
	})

	// 测试用例7: 并发访问
	t.Run("Concurrent Access", func(t *testing.T) {
		const goroutines = 100
		done := make(chan bool)

		for i := 0; i < goroutines; i++ {
			go func(index int) {
				key := fmt.Sprintf("concurrent_key_%d", index)
				cache.Set(key, index)
				value, ok := cache.GetAndRefreshExpire(key, nil)
				assert.True(t, ok)
				assert.Equal(t, index, value)
				done <- true
			}(i)
		}

		for i := 0; i < goroutines; i++ {
			<-done
		}
	})
}
