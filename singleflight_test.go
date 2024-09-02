package ggcache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestGroup_Do_SingleCall(t *testing.T) {
	g := NewGroup[string, int]()

	calls := 0
	loader := func(key string, ctx context.Context) (int, error) {
		calls++
		return 42, nil
	}

	val, called, err := g.Do("key1", loader, context.Background())

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !called {
		t.Error("Expected loader to be called")
	}
	if val != 42 {
		t.Errorf("Expected value 42, got %d", val)
	}
	if calls != 1 {
		t.Errorf("Expected 1 call, got %d", calls)
	}
}

func TestGroup_Do_ConcurrentCalls(t *testing.T) {
	g := NewGroup[string, int]()

	var mu sync.Mutex
	calls := 0
	loader := func(key string, ctx context.Context) (int, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		time.Sleep(100 * time.Millisecond)
		return 42, nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			val, _, err := g.Do("key1", loader, context.Background())
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if val != 42 {
				t.Errorf("Expected value 42, got %d", val)
			}
		}()
	}
	wg.Wait()

	if calls != 1 {
		t.Errorf("Expected 1 call, got %d", calls)
	}
}

func TestGroup_Do_Error(t *testing.T) {
	g := NewGroup[string, int]()

	loader := func(key string, ctx context.Context) (int, error) {
		return 0, errors.New("test error")
	}

	val, called, err := g.Do("key1", loader, context.Background())

	if err == nil || err.Error() != "test error" {
		t.Errorf("Expected 'test error', got %v", err)
	}
	if !called {
		t.Error("Expected loader to be called")
	}
	if val != 0 {
		t.Errorf("Expected value 0, got %d", val)
	}
}

func TestGroup_Do_ContextCancellation(t *testing.T) {
	g := NewGroup[string, int]()

	loader := func(key string, ctx context.Context) (int, error) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
		return 42, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	val, called, err := g.Do("key1", loader, ctx)

	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded error, got %v", err)
	}
	if !called {
		t.Error("Expected loader to be called")
	}
	if val != 0 {
		t.Errorf("Expected value 0, got %d", val)
	}
}

func TestGroup_Do_MultipleKeys(t *testing.T) {
	g := NewGroup[string, int]()

	calls := make(map[string]int)
	var mu sync.Mutex
	loader := func(key string, ctx context.Context) (int, error) {
		mu.Lock()
		calls[key]++
		mu.Unlock()
		time.Sleep(100 * time.Millisecond)
		return len(key), nil
	}

	var wg sync.WaitGroup
	for _, key := range []string{"a", "bb", "ccc", "a", "bb", "ccc"} {
		wg.Add(1)
		go func(k string) {
			defer wg.Done()
			val, _, err := g.Do(k, loader, context.Background())
			if err != nil {
				t.Errorf("Unexpected error for key %s: %v", k, err)
			}
			if val != len(k) {
				t.Errorf("Expected value %d for key %s, got %d", len(k), k, val)
			}
		}(key)
	}
	wg.Wait()

	for key, count := range calls {
		if count != 1 {
			t.Errorf("Expected 1 call for key %s, got %d", key, count)
		}
	}
}

func TestGroup_Do_LargeNumberOfCalls(t *testing.T) {
	g := NewGroup[int, int]()

	calls := 0
	var mu sync.Mutex
	loader := func(key int, ctx context.Context) (int, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		time.Sleep(100 * time.Millisecond)
		return key * 2, nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			val, _, err := g.Do(k%10, loader, context.Background())
			if err != nil {
				t.Errorf("Unexpected error for key %d: %v", k, err)
			}
			if val != (k%10)*2 {
				t.Errorf("Expected value %d for key %d, got %d", (k%10)*2, k, val)
			}
		}(i)
	}
	wg.Wait()

	if calls != 10 {
		t.Errorf("Expected 10 calls, got %d", calls)
	}
}

// Test basic functionality: single call
func TestGroup_Do_SingleCall2(t *testing.T) {
	g := NewGroup[string, string]()
	loader := func(key string, ctx context.Context) (string, error) {
		return "value", nil
	}

	val, loaderCalled, err := g.Do("key", loader, context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "value" {
		t.Fatalf("expected value 'value', got %v", val)
	}
	if !loaderCalled {
		t.Fatalf("expected loader to be called")
	}
}

// Test concurrent calls with the same key
func TestGroup_Do_ConcurrentCalls2(t *testing.T) {
	g := NewGroup[string, string]()
	loader := func(key string, ctx context.Context) (string, error) {
		time.Sleep(100 * time.Millisecond)
		return "value", nil
	}

	var wg sync.WaitGroup
	const n = 3000
	results := make([]string, n)
	loaderCalled := make([]bool, n)
	errors := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], loaderCalled[i], errors[i] = g.Do("key", loader, context.Background())
		}(i)
	}

	wg.Wait()

	for i := 0; i < n; i++ {
		if errors[i] != nil {
			t.Fatalf("unexpected error: %v", errors[i])
		}
		if results[i] != "value" {
			t.Fatalf("expected value 'value', got %v", results[i])
		}
	}

	// Only one call should have actually called the loader
	loaderCallCount := 0
	for _, called := range loaderCalled {
		if called {
			loaderCallCount++
		}
	}
	if loaderCallCount != 1 {
		t.Fatalf("expected loader to be called once, but was called %d times", loaderCallCount)
	}
}

// Test concurrent calls with the same key
func TestGroup_Do_ConcurrentCalls3(t *testing.T) {
	g := NewGroup[string, string]()
	loader := func(key string, ctx context.Context) (string, error) {
		time.Sleep(100 * time.Millisecond)
		return "value", nil
	}

	var wg sync.WaitGroup
	const n = 2000
	results := make([]string, n)
	loaderCalled := make([]bool, n)
	errors := make([]error, n)
	startSignal := make(chan struct{})

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-startSignal
			results[i], loaderCalled[i], errors[i] = g.Do("key", loader, context.Background())
		}(i)
	}

	close(startSignal)
	wg.Wait()

	for i := 0; i < n; i++ {
		if errors[i] != nil {
			t.Fatalf("unexpected error: %v", errors[i])
		}
		if results[i] != "value" {
			t.Fatalf("expected value 'value', got %v", results[i])
		}
	}

	// Only one call should have actually called the loader
	loaderCallCount := 0
	for _, called := range loaderCalled {
		if called {
			loaderCallCount++
		}
	}
	if loaderCallCount != 1 {
		t.Fatalf("expected loader to be called once, but was called %d times", loaderCallCount)
	}
}

// Test error handling
func TestGroup_Do_Error2(t *testing.T) {
	g := NewGroup[string, string]()
	expectedErr := errors.New("loader error")
	loader := func(key string, ctx context.Context) (string, error) {
		return "", expectedErr
	}

	_, _, err := g.Do("key", loader, context.Background())
	if err != expectedErr {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
}

func TestGroup_Do_PanicHandling(t *testing.T) {
	group := NewGroup[string, int]()

	loader := func(key string, ctx context.Context) (int, error) {
		panic("simulated panic")
	}

	_, loaderCalled, err := group.Do("key1", loader, context.Background())
	if !loaderCalled {
		t.Errorf("expected loader to be called")
	}
	if err == nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGroup_Do_ErrorHandling(t *testing.T) {
	group := NewGroup[string, int]()

	expectedErr := errors.New("simulated error")
	loader := func(key string, ctx context.Context) (int, error) {
		return 0, expectedErr
	}

	_, loaderCalled, err := group.Do("key1", loader, context.Background())
	if !loaderCalled {
		t.Errorf("expected loader to be called")
	}
	if err == nil || err != expectedErr {
		t.Errorf("unexpected error: %v", err)
	}
}
