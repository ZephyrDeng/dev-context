package cache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestCacheStorage_BasicOperations(t *testing.T) {
	storage := NewCacheStorage(1024 * 1024) // 1MB
	
	// Test Set and Get
	err := storage.Set("test-key", "test-value", 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}
	
	result, found := storage.Get("test-key")
	if !found {
		t.Fatal("Cache item not found")
	}
	
	if result.Data != "test-value" {
		t.Fatalf("Expected 'test-value', got %v", result.Data)
	}
	
	// Test Size
	if storage.Size() != 1 {
		t.Fatalf("Expected size 1, got %d", storage.Size())
	}
	
	// Test Delete
	deleted := storage.Delete("test-key")
	if !deleted {
		t.Fatal("Failed to delete cache item")
	}
	
	if storage.Size() != 0 {
		t.Fatalf("Expected size 0 after delete, got %d", storage.Size())
	}
}

func TestCacheStorage_Expiration(t *testing.T) {
	storage := NewCacheStorage(1024 * 1024)
	
	// Set item with short TTL
	err := storage.Set("expire-key", "expire-value", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}
	
	// Should exist immediately
	_, found := storage.Get("expire-key")
	if !found {
		t.Fatal("Cache item should exist immediately after set")
	}
	
	// Wait for expiration
	time.Sleep(200 * time.Millisecond)
	
	// Should be expired
	_, found = storage.Get("expire-key")
	if found {
		t.Fatal("Cache item should be expired")
	}
}

func TestCacheStorage_GenerateKey(t *testing.T) {
	params1 := map[string]interface{}{
		"param1": "value1",
		"param2": 123,
	}
	
	params2 := map[string]interface{}{
		"param2": 123,
		"param1": "value1",
	}
	
	key1 := GenerateKey("query1", params1, "2023-01-01")
	key2 := GenerateKey("query1", params2, "2023-01-01")
	
	// Same parameters in different order should generate same key
	if key1 != key2 {
		t.Fatal("Keys should be same for same parameters in different order")
	}
	
	key3 := GenerateKey("query2", params1, "2023-01-01")
	if key1 == key3 {
		t.Fatal("Keys should be different for different query types")
	}
}

func TestCacheStorage_ConcurrentAccess(t *testing.T) {
	storage := NewCacheStorage(1024 * 1024)
	const numGoroutines = 100
	const numOperations = 100
	
	var wg sync.WaitGroup
	
	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				value := fmt.Sprintf("value-%d-%d", id, j)
				err := storage.Set(key, value, 5*time.Minute)
				if err != nil {
					t.Errorf("Failed to set cache: %v", err)
					return
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	expectedSize := numGoroutines * numOperations
	if storage.Size() != expectedSize {
		t.Fatalf("Expected size %d, got %d", expectedSize, storage.Size())
	}
	
	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				result, found := storage.Get(key)
				if !found {
					t.Errorf("Cache item not found: %s", key)
					return
				}
				expectedValue := fmt.Sprintf("value-%d-%d", id, j)
				if result.Data != expectedValue {
					t.Errorf("Expected %s, got %v", expectedValue, result.Data)
					return
				}
			}
		}(i)
	}
	
	wg.Wait()
}

func TestCacheManager_BasicOperations(t *testing.T) {
	config := &CacheConfig{
		MaxSize:         1024 * 1024,
		TTL:             5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}
	
	cm := NewCacheManager(config)
	defer cm.Close()
	
	// Test Set and Get
	err := cm.Set("test-key", "test-value")
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}
	
	value, found := cm.Get("test-key")
	if !found {
		t.Fatal("Cache item not found")
	}
	
	if value != "test-value" {
		t.Fatalf("Expected 'test-value', got %v", value)
	}
	
	// Test metrics
	stats := cm.GetStats()
	if stats["hits"].(int64) != 1 {
		t.Fatalf("Expected 1 hit, got %v", stats["hits"])
	}
	if stats["sets"].(int64) != 1 {
		t.Fatalf("Expected 1 set, got %v", stats["sets"])
	}
}

func TestCacheManager_GetOrSet(t *testing.T) {
	cm := NewCacheManager(nil)
	defer cm.Close()
	
	ctx := context.Background()
	key := "test-key"
	expectedValue := "computed-value"
	
	callCount := 0
	computeFunc := func(ctx context.Context) (interface{}, error) {
		callCount++
		time.Sleep(100 * time.Millisecond) // Simulate work
		return expectedValue, nil
	}
	
	// First call should compute and cache
	value, err := cm.GetOrSet(ctx, key, computeFunc)
	if err != nil {
		t.Fatalf("GetOrSet failed: %v", err)
	}
	if value != expectedValue {
		t.Fatalf("Expected %s, got %v", expectedValue, value)
	}
	if callCount != 1 {
		t.Fatalf("Expected function to be called once, called %d times", callCount)
	}
	
	// Second call should use cache
	value, err = cm.GetOrSet(ctx, key, computeFunc)
	if err != nil {
		t.Fatalf("GetOrSet failed: %v", err)
	}
	if value != expectedValue {
		t.Fatalf("Expected %s, got %v", expectedValue, value)
	}
	if callCount != 1 {
		t.Fatalf("Expected function to be called once, called %d times", callCount)
	}
}

func TestCacheManager_QueryCoalescing(t *testing.T) {
	cm := NewCacheManager(nil)
	defer cm.Close()
	
	ctx := context.Background()
	key := "coalesce-key"
	expectedValue := "coalesced-value"
	
	callCount := 0
	computeFunc := func(ctx context.Context) (interface{}, error) {
		callCount++
		time.Sleep(200 * time.Millisecond) // Simulate slow work
		return expectedValue, nil
	}
	
	const numConcurrent = 10
	var wg sync.WaitGroup
	results := make([]interface{}, numConcurrent)
	errors := make([]error, numConcurrent)
	
	// Launch concurrent requests for same key
	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index], errors[index] = cm.GetOrSet(ctx, key, computeFunc)
		}(i)
	}
	
	wg.Wait()
	
	// All should succeed
	for i := 0; i < numConcurrent; i++ {
		if errors[i] != nil {
			t.Fatalf("Request %d failed: %v", i, errors[i])
		}
		if results[i] != expectedValue {
			t.Fatalf("Request %d got wrong value: %v", i, results[i])
		}
	}
	
	// Function should only be called once due to coalescing
	if callCount != 1 {
		t.Fatalf("Expected function to be called once due to coalescing, called %d times", callCount)
	}
}

func TestCacheManager_Warmup(t *testing.T) {
	cm := NewCacheManager(nil)
	defer cm.Close()
	
	ctx := context.Background()
	warmupData := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	
	err := cm.Warmup(ctx, warmupData)
	if err != nil {
		t.Fatalf("Warmup failed: %v", err)
	}
	
	// All items should be accessible
	for key, expectedValue := range warmupData {
		value, found := cm.Get(key)
		if !found {
			t.Fatalf("Warmed up item not found: %s", key)
		}
		if value != expectedValue {
			t.Fatalf("Expected %v, got %v for key %s", expectedValue, value, key)
		}
	}
}

func TestCacheManager_BackgroundCleanup(t *testing.T) {
	config := &CacheConfig{
		MaxSize:         1024 * 1024,
		TTL:             100 * time.Millisecond, // Short TTL
		CleanupInterval: 50 * time.Millisecond,  // Frequent cleanup
	}
	
	cm := NewCacheManager(config)
	defer cm.Close()
	
	// Set items
	for i := 0; i < 5; i++ {
		err := cm.Set(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
		if err != nil {
			t.Fatalf("Failed to set cache: %v", err)
		}
	}
	
	if cm.Size() != 5 {
		t.Fatalf("Expected 5 items, got %d", cm.Size())
	}
	
	// Wait for items to expire and be cleaned up
	time.Sleep(300 * time.Millisecond)
	
	// Items should be cleaned up
	if cm.Size() != 0 {
		t.Fatalf("Expected 0 items after cleanup, got %d", cm.Size())
	}
}

func TestCacheStorage_LRUEviction(t *testing.T) {
	// Create storage with very small size to trigger eviction
	storage := NewCacheStorage(1024) // 1KB
	
	// Add items that will exceed the size limit
	for i := 0; i < 10; i++ {
		// Create large strings to trigger eviction
		largeString := fmt.Sprintf("large-value-%d-%s", i, 
			"this is a large string to consume memory and trigger eviction policies in the cache system")
		err := storage.Set(fmt.Sprintf("key%d", i), largeString, 5*time.Minute)
		if err != nil {
			t.Fatalf("Failed to set cache item %d: %v", i, err)
		}
	}
	
	// Should have fewer than 10 items due to eviction
	size := storage.Size()
	if size >= 10 {
		t.Logf("Size: %d, TotalSize: %d, MaxSize: %d", size, storage.TotalSize(), storage.MaxSize())
		t.Fatal("Expected LRU eviction to remove some items")
	}
	
	// Verify total size doesn't exceed limit
	if storage.TotalSize() > storage.MaxSize() {
		t.Fatalf("Total size %d exceeds max size %d", storage.TotalSize(), storage.MaxSize())
	}
}

func BenchmarkCacheStorage_Get(b *testing.B) {
	storage := NewCacheStorage(1024 * 1024)
	storage.Set("benchmark-key", "benchmark-value", 5*time.Minute)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = storage.Get("benchmark-key")
	}
}

func BenchmarkCacheStorage_Set(b *testing.B) {
	storage := NewCacheStorage(1024 * 1024)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = storage.Set(fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i), 5*time.Minute)
	}
}

func BenchmarkCacheManager_GetOrSet(b *testing.B) {
	cm := NewCacheManager(nil)
	defer cm.Close()
	
	ctx := context.Background()
	computeFunc := func(ctx context.Context) (interface{}, error) {
		return "computed-value", nil
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cm.GetOrSet(ctx, fmt.Sprintf("key-%d", i), computeFunc)
	}
}