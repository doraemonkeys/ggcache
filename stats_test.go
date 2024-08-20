package ggcache

import (
	"sync"
	"testing"
)

func TestStatsHitCount(t *testing.T) {
	s := &stats{}
	if got := s.HitCount(); got != 0 {
		t.Errorf("Initial HitCount = %v, want 0", got)
	}

	s.incrHitCount()
	if got := s.HitCount(); got != 1 {
		t.Errorf("HitCount after increment = %v, want 1", got)
	}
}

func TestStatsMissCount(t *testing.T) {
	s := &stats{}
	if got := s.MissCount(); got != 0 {
		t.Errorf("Initial MissCount = %v, want 0", got)
	}

	s.incrMissCount()
	if got := s.MissCount(); got != 1 {
		t.Errorf("MissCount after increment = %v, want 1", got)
	}
}

func TestStatsLookupCount(t *testing.T) {
	s := &stats{}
	s.incrHitCount()
	s.incrMissCount()
	if got := s.LookupCount(); got != 2 {
		t.Errorf("LookupCount = %v, want 2", got)
	}
}

func TestStatsHitRate(t *testing.T) {
	s := &stats{}
	if got := s.HitRate(); got != 0.0 {
		t.Errorf("Initial HitRate = %v, want 0.0", got)
	}

	s.incrHitCount()
	s.incrMissCount()
	if got := s.HitRate(); got != 0.5 {
		t.Errorf("HitRate = %v, want 0.5", got)
	}
}

func TestStatsConcurrency(t *testing.T) {
	s := &stats{}
	var wg sync.WaitGroup
	n := 1000

	wg.Add(n * 2)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			s.incrHitCount()
		}()
		go func() {
			defer wg.Done()
			s.incrMissCount()
		}()
	}
	wg.Wait()

	if got := s.HitCount(); got != uint64(n) {
		t.Errorf("HitCount = %v, want %v", got, n)
	}
	if got := s.MissCount(); got != uint64(n) {
		t.Errorf("MissCount = %v, want %v", got, n)
	}
	if got := s.LookupCount(); got != uint64(2*n) {
		t.Errorf("LookupCount = %v, want %v", got, 2*n)
	}
	if got := s.HitRate(); got != 0.5 {
		t.Errorf("HitRate = %v, want 0.5", got)
	}
}
