package cache

import (
	"testing"
	"time"
)

// TestCacheManagerWithMetricsIntegration 测试缓存管理器与指标系统的集成
func TestCacheManagerWithMetricsIntegration(t *testing.T) {
	// 创建缓存管理器
	config := DefaultCacheConfig()
	cm := NewCacheManager(config)
	defer cm.Close()
	
	// 创建详细指标
	metrics := NewDetailedCacheMetrics(config.MaxSize)
	
	// 模拟一些缓存操作并记录指标
	testData := map[string]interface{}{
		"key1": "value1",
		"key2": []byte("binary_data"),
		"key3": 12345,
		"key4": "large_value_" + string(make([]byte, 1000)), // 大值测试
	}
	
	// 记录设置操作
	for key, value := range testData {
		start := time.Now()
		err := cm.Set(key, value)
		duration := time.Since(start)
		
		if err != nil {
			t.Errorf("Failed to set key %s: %v", key, err)
		}
		
		// 记录指标
		dataType := "unknown"
		switch value.(type) {
		case string:
			dataType = "string"
		case []byte:
			dataType = "[]byte"
		case int:
			dataType = "int"
		}
		
		size := int64(100) // 简化的大小估算
		if key == "key4" {
			size = 1100
		}
		
		metrics.RecordSet(key, duration, size, dataType)
	}
	
	// 记录获取操作（包括命中和未命中）
	allKeys := []string{"key1", "key2", "key3", "key4", "nonexistent"}
	
	for _, key := range allKeys {
		start := time.Now()
		value, found := cm.Get(key)
		duration := time.Since(start)
		
		size := int64(100)
		if key == "key4" {
			size = 1100
		}
		
		metrics.RecordGet(key, found, duration, size)
		
		if key != "nonexistent" && !found {
			t.Errorf("Expected to find key %s", key)
		}
		if key == "nonexistent" && found {
			t.Errorf("Should not find nonexistent key")
		}
		
		// 验证数据正确性
		if found && key == "key1" {
			if value != "value1" {
				t.Errorf("Expected value1 for key1, got %v", value)
			}
		}
	}
	
	// 删除一个键
	start := time.Now()
	deleted := cm.Delete("key1")
	duration := time.Since(start)
	
	if !deleted {
		t.Error("Expected key1 to be deleted")
	}
	
	metrics.RecordDelete("key1", duration, 100)
	
	// 获取并验证统计信息
	stats := metrics.GetDetailedStats()
	
	// 验证基本计数
	if hits := stats["hits"].(int64); hits != 4 {
		t.Errorf("Expected 4 hits, got %d", hits)
	}
	
	if misses := stats["misses"].(int64); misses != 1 {
		t.Errorf("Expected 1 miss, got %d", misses)
	}
	
	if sets := stats["sets"].(int64); sets != 4 {
		t.Errorf("Expected 4 sets, got %d", sets)
	}
	
	if deletes := stats["deletes"].(int64); deletes != 1 {
		t.Errorf("Expected 1 delete, got %d", deletes)
	}
	
	// 验证命中率
	expectedHitRate := float64(4) / float64(5) * 100 // 4 hits out of 5 gets
	actualHitRate := stats["hit_rate"].(float64)
	if actualHitRate != expectedHitRate {
		t.Errorf("Expected hit rate %.2f%%, got %.2f%%", expectedHitRate, actualHitRate)
	}
	
	// 验证类型分布
	typeDistribution := stats["type_distribution"].(map[string]int64)
	if typeDistribution["string"] != 2 { // key1 and key4
		t.Errorf("Expected 2 string types, got %d", typeDistribution["string"])
	}
	
	if typeDistribution["[]byte"] != 1 {
		t.Errorf("Expected 1 []byte type, got %d", typeDistribution["[]byte"])
	}
	
	if typeDistribution["int"] != 1 {
		t.Errorf("Expected 1 int type, got %d", typeDistribution["int"])
	}
	
	// 验证大小分布（注意：每个操作都被记录，包括get操作）
	sizeDistribution := stats["size_distribution"].(map[string]int64)
	
	// 计算总的大小分布项数
	totalSizeItems := int64(0)
	for _, count := range sizeDistribution {
		totalSizeItems += count
	}
	
	// 应该有: 4个set + 5个get = 9个操作被记录大小分布（delete不记录大小分布）
	expectedTotal := int64(9)
	if totalSizeItems != expectedTotal {
		t.Logf("Size distribution items: %v", sizeDistribution)
		t.Errorf("Expected %d total size distribution items, got %d", expectedTotal, totalSizeItems)
	}
	
	// 验证访问频率统计
	topKeys := stats["top_accessed_keys"].([]map[string]interface{})
	if len(topKeys) == 0 {
		t.Error("Expected some top accessed keys")
	}
	
	// 验证响应时间统计存在
	if getResponseTime, exists := stats["get_response_time"]; !exists {
		t.Error("Expected get response time stats")
	} else {
		getStats := getResponseTime.(map[string]interface{})
		if count := getStats["count"].(int64); count != 5 {
			t.Errorf("Expected 5 get operations recorded, got %d", count)
		}
	}
	
	if setResponseTime, exists := stats["set_response_time"]; !exists {
		t.Error("Expected set response time stats")
	} else {
		setStats := setResponseTime.(map[string]interface{})
		if count := setStats["count"].(int64); count != 4 {
			t.Errorf("Expected 4 set operations recorded, got %d", count)
		}
	}
}

// TestMonitoringIntegration 测试监控系统集成
func TestMonitoringIntegration(t *testing.T) {
	// 创建完整的缓存和监控系统
	config := DefaultCacheConfig()
	cm := NewCacheManager(config)
	defer cm.Close()
	
	metrics := NewDetailedCacheMetrics(config.MaxSize)
	
	monitoringConfig := DefaultMonitoringConfig()
	monitoringConfig.EnableHTTPEndpoints = false // 测试时禁用HTTP
	monitoringConfig.MetricsInterval = 0 // 禁用定期收集
	monitoringConfig.HealthCheckInterval = 0 // 禁用健康检查
	
	monitor := NewCacheMonitor(cm, metrics, monitoringConfig)
	
	// 测试告警系统
	alertReceived := false
	var receivedAlert *Alert
	
	monitor.AddAlertCallback(func(alert *Alert) {
		alertReceived = true
		receivedAlert = alert
	})
	
	// 模拟低命中率情况来触发告警
	stats := map[string]interface{}{
		"hit_rate":                10.0, // 低于默认阈值20%
		"memory_usage_percent":    50.0,
		"error_count":             int64(5),
		"slow_operation_count":    int64(10),
	}
	
	monitor.checkAlerts(stats)
	
	// 等待异步告警处理
	time.Sleep(10 * time.Millisecond)
	
	if !alertReceived {
		t.Error("Expected to receive alert for low hit rate")
	}
	
	if receivedAlert == nil {
		t.Error("Expected alert object")
	} else {
		if receivedAlert.Level != "warning" {
			t.Errorf("Expected warning level, got %s", receivedAlert.Level)
		}
	}
	
	// 测试调试信息
	debugInfo := monitor.GetDebugInfo()
	if debugInfo == nil {
		t.Error("Expected debug info")
	}
	
	if monitorStatus, exists := debugInfo["monitor_status"]; !exists {
		t.Error("Expected monitor_status in debug info")
	} else {
		status := monitorStatus.(map[string]interface{})
		if running := status["running"].(bool); running {
			t.Error("Expected monitor to not be running initially")
		}
	}
	
	// 测试配置管理
	originalPort := monitoringConfig.HTTPPort
	newConfig := *monitoringConfig
	newConfig.HTTPPort = 9999
	
	err := monitor.UpdateConfiguration(&newConfig)
	if err != nil {
		t.Errorf("Failed to update configuration: %v", err)
	}
	
	currentConfig := monitor.GetConfiguration()
	if currentConfig.HTTPPort != 9999 {
		t.Errorf("Expected updated port 9999, got %d", currentConfig.HTTPPort)
	}
	
	// 验证原始配置没有被修改
	if monitoringConfig.HTTPPort != originalPort {
		t.Error("Original config should not be modified")
	}
}

// TestCacheMetricsHealthStatus 测试健康状态评估
func TestCacheMetricsHealthStatus(t *testing.T) {
	metrics := NewDetailedCacheMetrics(1024)
	
	// 初始状态应该是健康或degraded（取决于命中率计算）
	initialHealth := metrics.GetHealthStatus()
	status := initialHealth["status"].(string)
	if status != "healthy" && status != "degraded" {
		t.Errorf("Expected healthy or degraded status initially, got %s", status)
	}
	
	// 建立一个良好的命中率
	for i := 0; i < 10; i++ {
		metrics.RecordGet("good_key", true, time.Millisecond, 100)
	}
	
	healthyStatus := metrics.GetHealthStatus()
	if healthyStatus["status"].(string) != "healthy" {
		t.Errorf("Expected healthy status with good hit rate, got %s", healthyStatus["status"])
	}
	
	// 模拟大量内存使用
	for i := 0; i < 100; i++ {
		metrics.MemoryUsage.RecordAllocation(10240) // 分配大量内存
	}
	
	criticalHealth := metrics.GetHealthStatus()
	criticalStatus := criticalHealth["status"].(string)
	if criticalStatus != "critical" && criticalStatus != "degraded" {
		t.Errorf("Expected critical or degraded status with high memory usage, got %s", criticalStatus)
	}
	
	warnings := criticalHealth["warnings"].([]string)
	if len(warnings) == 0 {
		t.Error("Expected warnings for high memory usage")
	}
}

// TestPerformanceUnderLoad 性能负载测试
func TestPerformanceUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}
	
	config := DefaultCacheConfig()
	cm := NewCacheManager(config)
	defer cm.Close()
	
	metrics := NewDetailedCacheMetrics(config.MaxSize)
	
	// 并发执行缓存操作
	numOperations := 1000
	
	start := time.Now()
	
	for i := 0; i < numOperations; i++ {
		key := "perf_key_" + string(rune(i%100))
		value := "perf_value_" + string(rune(i))
		
		opStart := time.Now()
		
		// 随机选择操作类型
		switch i % 4 {
		case 0, 1: // 50% set操作
			err := cm.Set(key, value)
			opDuration := time.Since(opStart)
			if err != nil {
				t.Errorf("Set operation failed: %v", err)
			}
			metrics.RecordSet(key, opDuration, 100, "string")
			
		case 2: // 25% get操作
			_, found := cm.Get(key)
			opDuration := time.Since(opStart)
			metrics.RecordGet(key, found, opDuration, 100)
			
		case 3: // 25% delete操作
			deleted := cm.Delete(key)
			opDuration := time.Since(opStart)
			if deleted {
				metrics.RecordDelete(key, opDuration, 100)
			}
		}
	}
	
	totalDuration := time.Since(start)
	
	// 获取性能统计
	stats := metrics.GetDetailedStats()
	
	// 验证操作完成
	totalOps := stats["hits"].(int64) + stats["misses"].(int64) + stats["sets"].(int64) + stats["deletes"].(int64)
	if totalOps == 0 {
		t.Error("Expected some operations to be recorded")
	}
	
	// 计算操作速率
	opsPerSecond := float64(totalOps) / totalDuration.Seconds()
	
	t.Logf("Performance results:")
	t.Logf("  Total operations: %d", totalOps)
	t.Logf("  Total duration: %v", totalDuration)
	t.Logf("  Operations per second: %.2f", opsPerSecond)
	t.Logf("  Hit rate: %.2f%%", stats["hit_rate"].(float64))
	
	// 基本性能要求
	if opsPerSecond < 1000 {
		t.Logf("Warning: Low operation rate: %.2f ops/sec", opsPerSecond)
	}
	
	// 验证响应时间合理
	getResponseTime := stats["get_response_time"].(map[string]interface{})
	if avgMs := getResponseTime["average_ms"].(float64); avgMs > 100 {
		t.Logf("Warning: High average get response time: %.2f ms", avgMs)
	}
	
	setResponseTime := stats["set_response_time"].(map[string]interface{})
	if avgMs := setResponseTime["average_ms"].(float64); avgMs > 100 {
		t.Logf("Warning: High average set response time: %.2f ms", avgMs)
	}
}