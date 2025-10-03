package tests

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/lmriccardo/synchme/internal/utils"
)

func TestNewCache(t *testing.T) {
	MaxTTL := 10 * time.Second
	cache := utils.NewCache(MaxTTL)

	if cache == nil {
		t.Fatal("NewCache returned nil")
	}
	if cache.MaxTTL != MaxTTL {
		t.Errorf("Expected MaxTTL %v, got %v", MaxTTL, cache.MaxTTL)
	}
	if cache.Items == nil {
		t.Error("Cache Items map is nil")
	}
}

func TestSetAndGet(t *testing.T) {
	MaxTTL := time.Minute
	cache := utils.NewCache(MaxTTL)

	key := "testKey"
	value := "testValue"
	ttl := 10 * time.Second

	// 1. Test Set and Get (started item)
	cache.Set(key, value, ttl, true)
	retrieved, ok := cache.Get(key)

	if !ok {
		t.Fatal("Get failed: expected cache hit, got miss")
	}
	if retrieved != value {
		t.Errorf("Get failed: expected value %q, got %q", value, retrieved)
	}

	// 2. Test Get (cache miss)
	_, okMiss := cache.Get("nonExistentKey")
	if okMiss {
		t.Error("Get failed: expected cache miss, got hit")
	}

	// 3. Test MaxTTL constraint
	veryLongTTL := 5 * time.Hour
	cache.Set("MaxTTLKey", "MaxTTLValue", veryLongTTL, true)
	if cache.Items["MaxTTLKey"].TimeToLive != MaxTTL {
		t.Errorf("Set failed to respect MaxTTL: expected %v, got %v", MaxTTL, cache.Items["MaxTTLKey"].TimeToLive)
	}
}

func TestExpiration(t *testing.T) {
	cache := utils.NewCache(time.Minute)
	shortTTL := 50 * time.Millisecond

	keyStarted := "expiringKey"
	keyUnstarted := "nonExpiringKey"

	cache.Set(keyStarted, "value", shortTTL, true)
	cache.Set(keyUnstarted, "value", shortTTL, false)

	// Check before expiration
	expired, _ := cache.HasExpired(keyStarted)
	if expired {
		t.Error("Started item should not have expired yet")
	}
	expired, _ = cache.HasExpired(keyUnstarted)
	if expired {
		t.Error("Unstarted item should not have expired")
	}

	// Wait for the item to expire
	time.Sleep(shortTTL + 20*time.Millisecond)

	// Check after expiration
	expired, err := cache.HasExpired(keyStarted)
	if !expired || err != nil {
		t.Errorf("Expected item to be expired (true, nil error), got (%v, %v)", expired, err)
	}

	// A Get call on an expired item should fail
	_, ok := cache.Get(keyStarted)
	if ok {
		t.Error("Get should return miss for an expired item")
	}

	// Unstarted item should still be valid
	_, okUnstarted := cache.Get(keyUnstarted)
	if !okUnstarted {
		t.Error("Unstarted item should not expire and be retrievable")
	}
}

func TestHitCountAndExtension(t *testing.T) {
	MaxTTL := 10 * time.Second
	baseTTL := 100 * time.Millisecond
	cache := utils.NewCache(MaxTTL)
	key := "extendingKey"

	cache.Set(key, "value", baseTTL, true)
	item := cache.GetWithNoHit(key)

	// 1. First Get (HitCount=1)
	cache.Get(key)
	if item.HitCount != 1 {
		t.Errorf("Expected HitCount 1, got %d", item.HitCount)
	}
	// Expected extension = baseTTL * ln(2) approx 69ms
	expectedExt1 := time.Duration(float64(baseTTL) * math.Log(2))
	if item.Expiration.Before(time.Now().Add(expectedExt1-10*time.Millisecond)) ||
		item.Expiration.After(time.Now().Add(expectedExt1+10*time.Millisecond)) {
		t.Errorf("HitCount 1: Expiration time not correctly extended by ~%v", expectedExt1)
	}

	// 2. Second Get (HitCount=2)
	cache.Get(key)
	if item.HitCount != 2 {
		t.Errorf("Expected HitCount 2, got %d", item.HitCount)
	}
	// Expected extension = baseTTL * ln(3) approx 109ms
	expectedExt2 := time.Duration(float64(baseTTL) * math.Log(3))
	if item.Expiration.Before(time.Now().Add(expectedExt2-10*time.Millisecond)) ||
		item.Expiration.After(time.Now().Add(expectedExt2+10*time.Millisecond)) {
		t.Errorf("HitCount 2: Expiration time not correctly extended by ~%v", expectedExt2)
	}

	// 3. Test Max TTL constraint
	// The calculated extension for 10 million hits (~1.61s) should be capped by a lower testMaxTTL.
	item.HitCount = 10000000
	testMaxTTL := 1 * time.Second // Max TTL set lower than the calculated extension (~1.61s)
	extMax := utils.ComputeNewExpiration(item, testMaxTTL)

	// Check that the calculated extension is capped at testMaxTTL
	if extMax != testMaxTTL {
		t.Errorf("Expected extension to be capped at testMaxTTL %v, got %v", testMaxTTL, extMax)
	}
}

func TestModify(t *testing.T) {
	cache := utils.NewCache(time.Minute)
	key := "modKey"
	initialValue := 100
	newValue := 200

	cache.Set(key, initialValue, 50*time.Millisecond, true)

	// 1. Modify existing key
	modified := cache.Modify(key, newValue)
	if !modified {
		t.Fatal("Modify failed to modify an existing key")
	}

	retrieved, _ := cache.Get(key)
	if retrieved != newValue {
		t.Errorf("Expected modified value %v, got %v", newValue, retrieved)
	}

	// 2. Modify non-existent key
	modified = cache.Modify("nonExistent", "new")
	if modified {
		t.Error("Modify unexpectedly modified a non-existent key")
	}
}

func TestRemoveAndErase(t *testing.T) {
	cache := utils.NewCache(time.Minute)
	cache.Set("key1", 1, time.Second, true)
	cache.Set("key2", 2, time.Second, true)

	// 1. Test Remove
	cache.Remove("key1")
	_, ok := cache.Get("key1")
	if ok {
		t.Error("Key1 should have been removed")
	}
	_, ok = cache.Get("key2")
	if !ok {
		t.Error("Key2 should still exist")
	}

	// 2. Test EraseCache
	cache.EraseCache()
	if len(cache.Items) != 0 {
		t.Errorf("Expected cache length 0 after EraseCache, got %d", len(cache.Items))
	}
}

func TestRun(t *testing.T) {
	cache := utils.NewCache(time.Minute)
	shortInterval := 20 * time.Millisecond
	shortTTL := 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cache.Run(ctx, shortInterval)

	key1 := "expired"
	key2 := "will_be_removed_on_cancel"

	cache.Set(key1, 1, shortTTL, true)
	cache.Set(key2, 2, time.Minute, true)

	// 1. Wait for key1 to expire and be cleaned up by the routine
	time.Sleep(shortTTL + shortInterval*2) // Wait for TTL + two routine runs

	_, ok1 := cache.Get(key1)
	if ok1 {
		t.Error("Run routine failed to remove expired key1")
	}

	// 2. Stop the routine and check for EraseCache cleanup
	cancel()
	time.Sleep(100 * time.Millisecond) // Give time for the Run routine to receive the ctx.Done() signal

	// key2 should now be removed by the final EraseCache call in Run
	exists, err := cache.HasExpired(key2)

	if !exists {
		t.Error("Run routine failed to EraseCache upon context cancellation: ", err)
	}
}
