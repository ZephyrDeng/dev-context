package cache

import (
	"fmt"
	"testing"
	"time"
)

func TestCleanupManager_BasicOperations(t *testing.T) {
	storage := NewCacheStorage(1024 * 1024) // 1MB
	ttlManager := NewTTLManager(DefaultTTLConfig())
	config := DefaultCleanupConfig()
	config.CleanupInterval = 1 * time.Hour // 防止自动清理干扰测试
	
	cleanup := NewCleanupManager(storage, ttlManager, config)
	defer cleanup.Close()
	defer ttlManager.Close()

	// 添加一些测试数据
	testData := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	for key, value := range testData {
		storage.Set(key, value, 10*time.Second)
		ttlManager.Set(key, 10*time.Second)
	}

	// 验证初始状态
	if storage.Size() != 3 {
		t.Errorf("Expected 3 items in storage, got %d", storage.Size())
	}

	// 测试强制清理（应该不清理任何东西，因为都没过期）
	cleanedKeys, err := cleanup.ForceCleanup()
	if err != nil {
		t.Errorf("ForceCleanup failed: %v", err)
	}

	if len(cleanedKeys) != 0 {
		t.Errorf("Expected 0 cleaned keys, got %d", len(cleanedKeys))
	}
}

func TestCleanupManager_TTLOnlyStrategy(t *testing.T) {
	storage := NewCacheStorage(1024 * 1024)
	
	// 创建具有较短最小TTL的TTL管理器
	ttlConfig := DefaultTTLConfig()
	ttlConfig.MinTTL = 10 * time.Millisecond
	ttlManager := NewTTLManager(ttlConfig)
	
	config := DefaultCleanupConfig()
	config.Strategy = StrategyTTLOnly
	config.CleanupInterval = 1 * time.Hour
	config.MaxCleanupPercent = 1.0 // 允许清理所有项目
	
	cleanup := NewCleanupManager(storage, ttlManager, config)
	defer cleanup.Close()
	defer ttlManager.Close()

	// 添加快速过期的项目
	expiredKeys := []string{"expire1", "expire2"}
	for _, key := range expiredKeys {
		storage.Set(key, "data", 50*time.Millisecond)
		ttlManager.Set(key, 50*time.Millisecond)
	}

	// 添加长期项目
	longKeys := []string{"keep1", "keep2"}
	for _, key := range longKeys {
		storage.Set(key, "data", 10*time.Second)
		ttlManager.Set(key, 10*time.Second)
	}

	// 等待过期
	time.Sleep(100 * time.Millisecond)

	// 执行清理
	cleanedKeys, err := cleanup.ForceCleanup()
	if err != nil {
		t.Errorf("ForceCleanup failed: %v", err)
	}

	// 验证只清理了过期项目
	if len(cleanedKeys) != 2 {
		t.Errorf("Expected 2 cleaned keys, got %d", len(cleanedKeys))
	}

	// 验证存储中只剩下长期项目
	if storage.Size() != 2 {
		t.Errorf("Expected 2 remaining items, got %d", storage.Size())
	}

	// 验证清理的是过期项目
	cleanedSet := make(map[string]bool)
	for _, key := range cleanedKeys {
		cleanedSet[key] = true
	}

	for _, key := range expiredKeys {
		if !cleanedSet[key] {
			t.Errorf("Expected key %s to be cleaned", key)
		}
	}
}

func TestCleanupManager_LRUStrategy(t *testing.T) {
	storage := NewCacheStorage(1024 * 1024)
	ttlManager := NewTTLManager(DefaultTTLConfig())
	config := DefaultCleanupConfig()
	config.Strategy = StrategyLRU
	config.CleanupInterval = 1 * time.Hour
	config.Threshold = map[string]interface{}{
		"max_age": 100 * time.Millisecond,
	}
	
	cleanup := NewCleanupManager(storage, ttlManager, config)
	defer cleanup.Close()
	defer ttlManager.Close()

	// 添加项目
	keys := []string{"old1", "old2", "recent1", "recent2"}
	for _, key := range keys {
		storage.Set(key, "data", 10*time.Second)
		ttlManager.Set(key, 10*time.Second)
	}

	// 等待让前两个项目变"旧"
	time.Sleep(150 * time.Millisecond)

	// 访问后两个项目使其"新鲜"
	storage.Get("recent1")
	storage.Get("recent2")

	// 执行清理
	cleanedKeys, err := cleanup.ForceCleanup()
	if err != nil {
		t.Errorf("ForceCleanup failed: %v", err)
	}

	// 应该清理旧项目
	if len(cleanedKeys) < 1 {
		t.Errorf("Expected at least 1 cleaned key, got %d", len(cleanedKeys))
	}

	// 验证新项目仍然存在
	for _, key := range []string{"recent1", "recent2"} {
		if _, exists := storage.Get(key); !exists {
			t.Errorf("Recent key %s should still exist", key)
		}
	}
}

func TestCleanupManager_MixedStrategy(t *testing.T) {
	storage := NewCacheStorage(1024 * 1024)
	ttlManager := NewTTLManager(DefaultTTLConfig())
	config := DefaultCleanupConfig()
	config.Strategy = StrategyMixed
	config.CleanupInterval = 1 * time.Hour
	config.Threshold = map[string]interface{}{
		"max_age":  200 * time.Millisecond,
		"min_size": int64(100),
	}
	
	cleanup := NewCleanupManager(storage, ttlManager, config)
	defer cleanup.Close()
	defer ttlManager.Close()

	// 添加过期项目（优先级最高）
	storage.Set("expired", "data", 50*time.Millisecond)
	ttlManager.Set("expired", 50*time.Millisecond)

	// 添加旧项目
	storage.Set("old", "data", 10*time.Second)
	ttlManager.Set("old", 10*time.Second)

	// 添加新项目
	storage.Set("new", "data", 10*time.Second)
	ttlManager.Set("new", 10*time.Second)

	// 等待过期和LRU条件触发
	time.Sleep(250 * time.Millisecond)

	// 保持新项目活跃
	storage.Get("new")

	// 执行清理
	cleanedKeys, err := cleanup.ForceCleanup()
	if err != nil {
		t.Errorf("ForceCleanup failed: %v", err)
	}

	// 应该至少清理过期项目
	if len(cleanedKeys) < 1 {
		t.Errorf("Expected at least 1 cleaned key, got %d", len(cleanedKeys))
	}

	// 新项目应该仍然存在
	if _, exists := storage.Get("new"); !exists {
		t.Error("New key should still exist")
	}
}

func TestCleanupManager_MaxCleanupPercent(t *testing.T) {
	storage := NewCacheStorage(1024 * 1024)
	ttlManager := NewTTLManager(DefaultTTLConfig())
	config := DefaultCleanupConfig()
	config.Strategy = StrategyTTLOnly
	config.CleanupInterval = 1 * time.Hour
	config.MaxCleanupPercent = 0.5 // 最多清理50%
	
	cleanup := NewCleanupManager(storage, ttlManager, config)
	defer cleanup.Close()
	defer ttlManager.Close()

	// 添加10个快速过期项目
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("expire_%d", i)
		storage.Set(key, "data", 50*time.Millisecond)
		ttlManager.Set(key, 50*time.Millisecond)
	}

	// 等待过期
	time.Sleep(100 * time.Millisecond)

	// 执行清理
	cleanedKeys, err := cleanup.ForceCleanup()
	if err != nil {
		t.Errorf("ForceCleanup failed: %v", err)
	}

	// 应该最多清理5个（50%）
	if len(cleanedKeys) > 5 {
		t.Errorf("Expected at most 5 cleaned keys due to percentage limit, got %d", len(cleanedKeys))
	}
}

func TestCleanupManager_StrategyChange(t *testing.T) {
	storage := NewCacheStorage(1024 * 1024)
	ttlManager := NewTTLManager(DefaultTTLConfig())
	config := DefaultCleanupConfig()
	config.Strategy = StrategyTTLOnly
	config.CleanupInterval = 1 * time.Hour
	
	cleanup := NewCleanupManager(storage, ttlManager, config)
	defer cleanup.Close()
	defer ttlManager.Close()

	// 测试获取和设置策略
	if cleanup.GetStrategy() != StrategyTTLOnly {
		t.Errorf("Expected initial strategy %v, got %v", StrategyTTLOnly, cleanup.GetStrategy())
	}

	cleanup.SetStrategy(StrategyLRU)
	if cleanup.GetStrategy() != StrategyLRU {
		t.Errorf("Expected strategy %v after setting, got %v", StrategyLRU, cleanup.GetStrategy())
	}
}

func TestCleanupManager_CleanupInterval(t *testing.T) {
	storage := NewCacheStorage(1024 * 1024)
	ttlManager := NewTTLManager(DefaultTTLConfig())
	config := DefaultCleanupConfig()
	initialInterval := 10 * time.Minute
	config.CleanupInterval = initialInterval
	
	cleanup := NewCleanupManager(storage, ttlManager, config)
	defer cleanup.Close()
	defer ttlManager.Close()

	// 测试获取清理间隔
	if cleanup.GetCleanupInterval() != initialInterval {
		t.Errorf("Expected initial interval %v, got %v", initialInterval, cleanup.GetCleanupInterval())
	}

	// 测试设置新的清理间隔
	newInterval := 5 * time.Minute
	cleanup.SetCleanupInterval(newInterval)

	if cleanup.GetCleanupInterval() != newInterval {
		t.Errorf("Expected interval %v after setting, got %v", newInterval, cleanup.GetCleanupInterval())
	}
}

func TestCleanupManager_ThresholdOperations(t *testing.T) {
	storage := NewCacheStorage(1024 * 1024)
	ttlManager := NewTTLManager(DefaultTTLConfig())
	config := DefaultCleanupConfig()
	initialThreshold := map[string]interface{}{
		"max_age": 5 * time.Minute,
		"min_size": int64(1000),
	}
	config.Threshold = initialThreshold
	
	cleanup := NewCleanupManager(storage, ttlManager, config)
	defer cleanup.Close()
	defer ttlManager.Close()

	// 测试获取阈值
	threshold := cleanup.GetThreshold()
	if threshold["max_age"].(time.Duration) != 5*time.Minute {
		t.Errorf("Expected max_age %v, got %v", 5*time.Minute, threshold["max_age"])
	}

	// 测试设置新阈值
	newThreshold := map[string]interface{}{
		"max_age": 2 * time.Minute,
		"min_size": int64(500),
	}
	cleanup.SetThreshold(newThreshold)

	updatedThreshold := cleanup.GetThreshold()
	if updatedThreshold["max_age"].(time.Duration) != 2*time.Minute {
		t.Errorf("Expected max_age %v after setting, got %v", 2*time.Minute, updatedThreshold["max_age"])
	}
}

func TestCleanupManager_MaxCleanupPercentOperations(t *testing.T) {
	storage := NewCacheStorage(1024 * 1024)
	ttlManager := NewTTLManager(DefaultTTLConfig())
	config := DefaultCleanupConfig()
	initialPercent := 0.3
	config.MaxCleanupPercent = initialPercent
	
	cleanup := NewCleanupManager(storage, ttlManager, config)
	defer cleanup.Close()
	defer ttlManager.Close()

	// 测试获取最大清理百分比
	if cleanup.GetMaxCleanupPercent() != initialPercent {
		t.Errorf("Expected initial percent %v, got %v", initialPercent, cleanup.GetMaxCleanupPercent())
	}

	// 测试设置新的百分比
	newPercent := 0.5
	cleanup.SetMaxCleanupPercent(newPercent)

	if cleanup.GetMaxCleanupPercent() != newPercent {
		t.Errorf("Expected percent %v after setting, got %v", newPercent, cleanup.GetMaxCleanupPercent())
	}

	// 测试边界值
	cleanup.SetMaxCleanupPercent(-0.1) // 应该被限制为0
	if cleanup.GetMaxCleanupPercent() != 0 {
		t.Error("Negative percent should be limited to 0")
	}

	cleanup.SetMaxCleanupPercent(1.5) // 应该被限制为1.0
	if cleanup.GetMaxCleanupPercent() != 1.0 {
		t.Error("Percent > 1.0 should be limited to 1.0")
	}
}

func TestCleanupManager_GetStats(t *testing.T) {
	storage := NewCacheStorage(1024 * 1024)
	
	// 设置适合的TTL配置
	ttlConfig := DefaultTTLConfig()
	ttlConfig.MinTTL = 1 * time.Millisecond
	ttlManager := NewTTLManager(ttlConfig)
	
	config := DefaultCleanupConfig()
	config.CleanupInterval = 1 * time.Hour
	config.MaxCleanupPercent = 1.0
	
	cleanup := NewCleanupManager(storage, ttlManager, config)
	defer cleanup.Close()
	defer ttlManager.Close()

	// 添加一些测试数据并执行清理
	storage.Set("expire", "data", 10*time.Millisecond)
	ttlManager.Set("expire", 10*time.Millisecond)
	
	time.Sleep(50 * time.Millisecond)
	
	cleanedKeys, _ := cleanup.ForceCleanup()

	// 获取统计信息
	stats := cleanup.GetStats()

	// 验证统计信息包含预期字段
	expectedFields := []string{
		"total_cleanups",
		"total_items_cleaned", 
		"total_space_freed",
		"current_strategy",
		"cleanup_interval",
		"max_cleanup_percent",
	}

	for _, field := range expectedFields {
		if _, exists := stats[field]; !exists {
			t.Errorf("Stats should include field %s", field)
		}
	}

	// 验证清理计数
	if stats["total_cleanups"].(int64) < 1 {
		t.Error("Should have recorded at least one cleanup")
	}

	if len(cleanedKeys) > 0 && stats["total_items_cleaned"].(int64) < 1 {
		t.Error("Should have recorded cleaned items")
	}
}

func TestCleanupManager_PredictNextCleanup(t *testing.T) {
	storage := NewCacheStorage(1024 * 1024)
	
	// 设置适合的TTL配置
	ttlConfig := DefaultTTLConfig()
	ttlConfig.MinTTL = 10 * time.Millisecond
	ttlManager := NewTTLManager(ttlConfig)
	
	config := DefaultCleanupConfig()
	config.Strategy = StrategyTTLOnly
	config.CleanupInterval = 1 * time.Hour
	config.MaxCleanupPercent = 1.0
	
	cleanup := NewCleanupManager(storage, ttlManager, config)
	defer cleanup.Close()
	defer ttlManager.Close()

	// 添加一些会过期的项目
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key_%d", i)
		storage.Set(key, "data", 50*time.Millisecond)
		ttlManager.Set(key, 50*time.Millisecond)
	}

	// 添加一些长期项目
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("keep_%d", i)
		storage.Set(key, "data", 10*time.Second)
		ttlManager.Set(key, 10*time.Second)
	}

	// 等待部分过期
	time.Sleep(100 * time.Millisecond)

	// 预测下次清理
	prediction := cleanup.PredictNextCleanup()

	// 验证预测信息
	expectedFields := []string{
		"total_items",
		"items_to_clean",
		"space_to_free",
		"cleanup_percentage",
		"next_cleanup_time",
	}

	for _, field := range expectedFields {
		if _, exists := prediction[field]; !exists {
			t.Errorf("Prediction should include field %s", field)
		}
	}

	// 验证预测的合理性
	if prediction["total_items"].(int) != 8 {
		t.Errorf("Expected 8 total items, got %v", prediction["total_items"])
	}

	if prediction["items_to_clean"].(int) < 1 {
		t.Error("Should predict cleaning at least some expired items")
	}
}

func TestCleanupManager_IsRunning(t *testing.T) {
	storage := NewCacheStorage(1024 * 1024)
	ttlManager := NewTTLManager(DefaultTTLConfig())
	config := DefaultCleanupConfig()
	
	cleanup := NewCleanupManager(storage, ttlManager, config)
	defer ttlManager.Close()

	// 应该正在运行
	if !cleanup.IsRunning() {
		t.Error("CleanupManager should be running after creation")
	}

	// 关闭后应该不在运行
	cleanup.Close()

	// 给一点时间让关闭完成
	time.Sleep(100 * time.Millisecond)

	if cleanup.IsRunning() {
		t.Error("CleanupManager should not be running after close")
	}
}

func TestCleanupItem_Priority(t *testing.T) {
	now := time.Now()
	
	// 创建测试项目
	item := &CleanupItem{
		Key:          "test",
		Size:         1024 * 100, // 100KB
		AccessCount:  5,
		LastAccessed: now.Add(-1 * time.Hour), // 1小时前访问
		CreatedAt:    now.Add(-2 * time.Hour), // 2小时前创建
		Expiry:       now.Add(1 * time.Hour),  // 1小时后过期
	}

	priority := item.CalculatePriority()

	// 非过期项目的优先级应该基于访问时间和大小
	if priority < 0 {
		t.Errorf("Expected non-negative priority for non-expired item, got %d", priority)
	}

	// 测试过期项目
	expiredItem := &CleanupItem{
		Key:          "expired",
		Size:         1024,
		AccessCount:  1,
		LastAccessed: now.Add(-1 * time.Hour),
		CreatedAt:    now.Add(-2 * time.Hour),
		Expiry:       now.Add(-30 * time.Minute), // 30分钟前就过期了
	}

	expiredPriority := expiredItem.CalculatePriority()

	// 过期项目应该有更高的优先级
	if expiredPriority <= priority {
		t.Errorf("Expired item should have higher priority: expired=%d, normal=%d", expiredPriority, priority)
	}
}

func TestCleanupItem_ShouldClean(t *testing.T) {
	now := time.Now()
	
	// 创建过期项目
	expiredItem := &CleanupItem{
		Key:          "expired",
		Size:         1024,
		AccessCount:  1,
		LastAccessed: now.Add(-1 * time.Hour),
		CreatedAt:    now.Add(-2 * time.Hour),
		Expiry:       now.Add(-30 * time.Minute),
	}

	// TTL策略下应该清理过期项目
	if !expiredItem.ShouldClean(StrategyTTLOnly, nil) {
		t.Error("Expired item should be cleaned with TTL strategy")
	}

	// 创建旧项目
	oldItem := &CleanupItem{
		Key:          "old",
		Size:         1024,
		AccessCount:  1,
		LastAccessed: now.Add(-2 * time.Hour), // 2小时前访问
		CreatedAt:    now.Add(-3 * time.Hour),
		Expiry:       now.Add(1 * time.Hour),
	}

	threshold := map[string]interface{}{
		"max_age": 1 * time.Hour, // 最大空闲1小时
	}

	// LRU策略下应该清理旧项目
	if !oldItem.ShouldClean(StrategyLRU, threshold) {
		t.Error("Old item should be cleaned with LRU strategy")
	}

	// 创建新项目
	newItem := &CleanupItem{
		Key:          "new",
		Size:         1024,
		AccessCount:  10,
		LastAccessed: now.Add(-30 * time.Minute), // 30分钟前访问
		CreatedAt:    now.Add(-1 * time.Hour),
		Expiry:       now.Add(1 * time.Hour),
	}

	// LRU策略下不应该清理新项目
	if newItem.ShouldClean(StrategyLRU, threshold) {
		t.Error("New item should not be cleaned with LRU strategy")
	}
}