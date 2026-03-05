package utils

import "testing"

func TestEstimateTokensWeighted_BasicBuckets(t *testing.T) {
	if got := EstimateTokensWeighted("", "abcd"); got != 1 {
		t.Fatalf("unexpected tokens for english: %d", got)
	}
	if got := EstimateTokensWeighted("", "中文中文"); got != 6 {
		t.Fatalf("unexpected tokens for cjk: %d", got)
	}
	if got := EstimateTokensWeighted("", "中文abcd"); got != 4 {
		t.Fatalf("unexpected tokens for mixed: %d", got)
	}
}

func TestTokenEstimateCache_RoundTrip(t *testing.T) {
	cache := NewTokenEstimateCache(2)
	key := TokenEstimateKey("gpt-4o", "hello")
	if _, ok := cache.Get(key); ok {
		t.Fatalf("expected miss")
	}
	cache.Put(key, 123)
	if v, ok := cache.Get(key); !ok || v != 123 {
		t.Fatalf("unexpected cache value: %v %v", v, ok)
	}
}
