package cache

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestTTLManager_BasicOperations(t *testing.T) {
	config := DefaultTTLConfig()
	config.MinTTL = 1 * time.Second // 设置更低的最小TTL以支持测试
	ttl := NewTTLManager(config)
	defer ttl.Close()

	key := "test_key"
	duration := 5 * time.Second

	// 测试设置TTL
	err := ttl.Set(key, duration)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// 测试获取TTL条目
	entry, exists := ttl.Get(key)
	if !exists {
		t.Error("Key should exist")
	}
	if entry.Key != key {
		t.Errorf("Expected key %s, got %s", key, entry.Key)
	}
	if entry.TTL != duration {
		t.Errorf("Expected TTL %v, got %v", duration, entry.TTL)
	}

	// 测试剩余TTL
	remaining, exists := ttl.GetRemainingTTL(key)
	if !exists {
		t.Error("Key should exist")
	}
	if remaining <= 0 || remaining > duration {
		t.Errorf("Invalid remaining TTL: %v", remaining)
	}

	// 测试是否过期
	if ttl.IsExpired(key) {
		t.Error("Key should not be expired")
	}

	// 测试删除
	deleted := ttl.Delete(key)
	if !deleted {
		t.Error("Delete should return true")
	}

	// 验证删除后状态
	_, exists = ttl.Get(key)
	if exists {
		t.Error("Key should not exist after deletion")
	}
}

func TestTTLManager_Expiration(t *testing.T) {
	config := DefaultTTLConfig()
	config.MinTTL = 50 * time.Millisecond
	ttl := NewTTLManager(config)
	defer ttl.Close()

	key := "expire_test"
	shortDuration := 100 * time.Millisecond

	// 设置很短的TTL
	err := ttl.Set(key, shortDuration)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// 等待过期
	time.Sleep(200 * time.Millisecond)

	// 检查是否过期
	if !ttl.IsExpired(key) {
		t.Error("Key should be expired")
	}

	// 剩余TTL应该为0
	remaining, exists := ttl.GetRemainingTTL(key)
	if exists || remaining != 0 {
		t.Errorf("Expected remaining TTL 0 and exists false, got %v, %v", remaining, exists)
	}
}

func TestTTLManager_ExtendTTL(t *testing.T) {
	config := DefaultTTLConfig()
	config.MinTTL = 500 * time.Millisecond
	ttl := NewTTLManager(config)
	defer ttl.Close()

	key := "extend_test"
	initialTTL := 1 * time.Second
	extension := 2 * time.Second

	// 设置初始TTL
	err := ttl.Set(key, initialTTL)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// 等待一段时间
	time.Sleep(500 * time.Millisecond)

	// 获取当前剩余时间
	beforeExtend, _ := ttl.GetRemainingTTL(key)

	// 延长TTL
	err = ttl.Extend(key, extension)
	if err != nil {
		t.Errorf("Extend failed: %v", err)
	}

	// 检查延长后的剩余时间
	afterExtend, exists := ttl.GetRemainingTTL(key)
	if !exists {
		t.Error("Key should still exist after extension")
	}

	// 延长后的时间应该大于延长前的时间
	if afterExtend <= beforeExtend {
		t.Errorf("TTL should be extended, before: %v, after: %v", beforeExtend, afterExtend)
	}
}

func TestTTLManager_RefreshTTL(t *testing.T) {
	config := DefaultTTLConfig()
	config.DefaultTTL = 2 * time.Second
	config.MinTTL = 1 * time.Second // 设置合适的最小值
	ttl := NewTTLManager(config)
	defer ttl.Close()

	key := "refresh_test"
	initialTTL := 1 * time.Second

	// 设置初始TTL
	err := ttl.Set(key, initialTTL)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// 等待一段时间
	time.Sleep(500 * time.Millisecond)

	// 刷新TTL（使用默认TTL）
	err = ttl.Refresh(key)
	if err != nil {
		t.Errorf("Refresh failed: %v", err)
	}

	// 检查刷新后的剩余时间
	remaining, exists := ttl.GetRemainingTTL(key)
	if !exists {
		t.Error("Key should exist after refresh")
	}

	// 剩余时间应该接近默认TTL
	if remaining < time.Duration(float64(time.Second)*1.8) || remaining > time.Duration(float64(time.Second)*2.1) {
		t.Errorf("Unexpected remaining TTL after refresh: %v", remaining)
	}

	// 测试使用自定义TTL刷新
	customTTL := 3 * time.Second
	err = ttl.Refresh(key, customTTL)
	if err != nil {
		t.Errorf("Refresh with custom TTL failed: %v", err)
	}

	remaining2, _ := ttl.GetRemainingTTL(key)
	if remaining2 < time.Duration(float64(time.Second)*2.8) || remaining2 > time.Duration(float64(time.Second)*3.1) {
		t.Errorf("Unexpected remaining TTL after custom refresh: %v", remaining2)
	}
}

func TestTTLManager_TTLLimits(t *testing.T) {
	config := DefaultTTLConfig()
	config.MinTTL = 1 * time.Second
	config.MaxTTL = 10 * time.Second
	ttl := NewTTLManager(config)
	defer ttl.Close()

	// 测试设置小于最小TTL的值
	err := ttl.Set("min_test", 500*time.Millisecond)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	entry, _ := ttl.Get("min_test")
	if entry.TTL != config.MinTTL {
		t.Errorf("Expected TTL to be limited to min value %v, got %v", config.MinTTL, entry.TTL)
	}

	// 测试设置大于最大TTL的值
	err = ttl.Set("max_test", 20*time.Second)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	entry, _ = ttl.Get("max_test")
	if entry.TTL != config.MaxTTL {
		t.Errorf("Expected TTL to be limited to max value %v, got %v", config.MaxTTL, entry.TTL)
	}
}

func TestTTLManager_CleanupExpired(t *testing.T) {
	config := DefaultTTLConfig()
	config.MinTTL = 50 * time.Millisecond
	ttl := NewTTLManager(config)
	defer ttl.Close()

	// 设置一些快速过期的键
	keys := []string{"expire1", "expire2", "expire3"}
	shortTTL := 100 * time.Millisecond

	for _, key := range keys {
		ttl.Set(key, shortTTL)
	}

	// 设置一些长期的键
	longKeys := []string{"keep1", "keep2"}
	longTTL := 10 * time.Second

	for _, key := range longKeys {
		ttl.Set(key, longTTL)
	}

	// 验证所有键都存在
	if ttl.Size() != 5 {
		t.Errorf("Expected 5 keys, got %d", ttl.Size())
	}

	// 等待短TTL键过期
	time.Sleep(200 * time.Millisecond)

	// 执行清理
	cleanedCount := ttl.CleanupExpired()
	if cleanedCount != 3 {
		t.Errorf("Expected to clean 3 keys, cleaned %d", cleanedCount)
	}

	// 验证只剩长期键
	if ttl.Size() != 2 {
		t.Errorf("Expected 2 keys after cleanup, got %d", ttl.Size())
	}

	// 验证长期键仍然存在
	for _, key := range longKeys {
		if ttl.IsExpired(key) {
			t.Errorf("Long-term key %s should not be expired", key)
		}
	}
}

func TestTTLManager_GetExpiredKeys(t *testing.T) {
	config := DefaultTTLConfig()
	config.MinTTL = 50 * time.Millisecond
	ttl := NewTTLManager(config)
	defer ttl.Close()

	// 设置一些键，部分会快速过期
	ttl.Set("expire1", 100*time.Millisecond)
	ttl.Set("expire2", 100*time.Millisecond)
	ttl.Set("keep1", 10*time.Second)
	ttl.Set("keep2", 10*time.Second)

	// 等待部分键过期
	time.Sleep(200 * time.Millisecond)

	// 获取过期键
	expiredKeys := ttl.GetExpiredKeys()
	if len(expiredKeys) != 2 {
		t.Errorf("Expected 2 expired keys, got %d", len(expiredKeys))
	}

	// 验证过期键的内容
	expiredSet := make(map[string]bool)
	for _, key := range expiredKeys {
		expiredSet[key] = true
	}

	if !expiredSet["expire1"] || !expiredSet["expire2"] {
		t.Error("Expected keys expire1 and expire2 to be in expired list")
	}
}

func TestTTLManager_GetStats(t *testing.T) {
	config := DefaultTTLConfig()
	config.MinTTL = 1 * time.Millisecond
	ttl := NewTTLManager(config)
	defer ttl.Close()

	// 添加一些键
	ttl.Set("key1", 5*time.Second)
	ttl.Set("key2", 10*time.Second)
	ttl.Set("key3", 1*time.Millisecond) // 会快速过期

	time.Sleep(10 * time.Millisecond) // 让一个键过期

	stats := ttl.GetStats()

	// 检查统计信息
	if stats["total_entries"].(int) != 3 {
		t.Errorf("Expected 3 total entries, got %v", stats["total_entries"])
	}

	if stats["expired_entries"].(int) != 1 {
		t.Errorf("Expected 1 expired entry, got %v", stats["expired_entries"])
	}

	if stats["active_entries"].(int) != 2 {
		t.Errorf("Expected 2 active entries, got %v", stats["active_entries"])
	}
}

func TestTTLManager_Concurrent(t *testing.T) {
	ttl := NewTTLManager(DefaultTTLConfig())
	defer ttl.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 并发设置和获取
	done := make(chan bool, 20)

	// 启动多个设置协程
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()
			key := fmt.Sprintf("key_%d", id)
			for {
				select {
				case <-ctx.Done():
					return
				default:
					ttl.Set(key, 1*time.Second)
					time.Sleep(10 * time.Millisecond)
				}
			}
		}(i)
	}

	// 启动多个获取协程
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()
			key := fmt.Sprintf("key_%d", id%5) // 访问一些相同的键
			for {
				select {
				case <-ctx.Done():
					return
				default:
					ttl.Get(key)
					ttl.GetRemainingTTL(key)
					ttl.IsExpired(key)
					time.Sleep(5 * time.Millisecond)
				}
			}
		}(i)
	}

	// 等待所有协程结束
	for i := 0; i < 20; i++ {
		<-done
	}

	// 验证没有数据竞争（如果有数据竞争，测试会失败或产生panic）
	stats := ttl.GetStats()
	if stats == nil {
		t.Error("Should be able to get stats after concurrent operations")
	}
}

func TestTTLManager_DefaultTTLOperations(t *testing.T) {
	config := DefaultTTLConfig()
	initialDefault := 5 * time.Second
	config.DefaultTTL = initialDefault
	config.MinTTL = 1 * time.Second // 设置合适的最小值
	config.MaxTTL = 10 * time.Second // 设置合适的最大值
	
	ttl := NewTTLManager(config)
	defer ttl.Close()

	// 测试获取默认TTL
	if ttl.GetDefaultTTL() != initialDefault {
		t.Errorf("Expected default TTL %v, got %v", initialDefault, ttl.GetDefaultTTL())
	}

	// 测试设置新的默认TTL
	newDefault := 8 * time.Second
	ttl.SetDefaultTTL(newDefault)

	if ttl.GetDefaultTTL() != newDefault {
		t.Errorf("Expected default TTL %v after setting, got %v", newDefault, ttl.GetDefaultTTL())
	}

	// 设置键时应该使用新的默认TTL
	ttl.Set("test", 0) // 0表示使用默认TTL

	entry, _ := ttl.Get("test")
	if entry.TTL != newDefault {
		t.Errorf("Expected TTL to be default %v, got %v", newDefault, entry.TTL)
	}
}

func TestTTLManager_ErrorCases(t *testing.T) {
	config := DefaultTTLConfig()
	config.MinTTL = 1 * time.Millisecond
	ttl := NewTTLManager(config)
	defer ttl.Close()

	// 测试延长不存在的键
	err := ttl.Extend("nonexistent", 1*time.Second)
	if err == nil {
		t.Error("Expected error when extending nonexistent key")
	}

	// 测试延长已过期的键
	ttl.Set("expired", 1*time.Millisecond)
	time.Sleep(10 * time.Millisecond)
	
	err = ttl.Extend("expired", 1*time.Second)
	if err == nil {
		t.Error("Expected error when extending expired key")
	}
}