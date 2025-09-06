package cache

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestResponseTimeStats(t *testing.T) {
	stats := NewResponseTimeStats(100)
	
	// 测试初始状态
	initialStats := stats.GetStats()
	if initialStats["count"].(int64) != 0 {
		t.Errorf("Expected initial count 0, got %v", initialStats["count"])
	}
	
	// 记录一些响应时间
	durations := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		200 * time.Millisecond,
	}
	
	for _, d := range durations {
		stats.Record(d)
	}
	
	// 验证统计结果
	result := stats.GetStats()
	
	if result["count"].(int64) != 5 {
		t.Errorf("Expected count 5, got %v", result["count"])
	}
	
	// 验证平均值
	avgMs := result["average_ms"].(float64)
	expectedAvg := float64((10 + 20 + 50 + 100 + 200) / 5)
	if avgMs != expectedAvg {
		t.Errorf("Expected average %f ms, got %f ms", expectedAvg, avgMs)
	}
	
	// 验证最小最大值
	minMs := result["min_ms"].(float64)
	maxMs := result["max_ms"].(float64)
	
	if minMs != 10.0 {
		t.Errorf("Expected min 10ms, got %f ms", minMs)
	}
	
	if maxMs != 200.0 {
		t.Errorf("Expected max 200ms, got %f ms", maxMs)
	}
	
	// 验证P50百分位数
	p50Ms := result["p50_ms"].(float64)
	if p50Ms != 50.0 {
		t.Errorf("Expected P50 50ms, got %f ms", p50Ms)
	}
}

func TestResponseTimeStatsSampleLimit(t *testing.T) {
	stats := NewResponseTimeStats(3)
	
	// 记录超过限制的样本
	for i := 0; i < 10; i++ {
		stats.Record(time.Duration(i+1) * time.Millisecond)
	}
	
	result := stats.GetStats()
	
	// 总计数应该是10
	if result["count"].(int64) != 10 {
		t.Errorf("Expected count 10, got %v", result["count"])
	}
	
	// 但样本数量应该被限制为3
	// P50应该基于最近的3个样本(8ms, 9ms, 10ms)
	p50Ms := result["p50_ms"].(float64)
	if p50Ms != 9.0 {
		t.Errorf("Expected P50 9ms (from recent samples), got %f ms", p50Ms)
	}
}

func TestResponseTimeStatsClear(t *testing.T) {
	stats := NewResponseTimeStats(100)
	
	// 添加一些数据
	stats.Record(10 * time.Millisecond)
	stats.Record(20 * time.Millisecond)
	
	// 验证有数据
	result := stats.GetStats()
	if result["count"].(int64) != 2 {
		t.Errorf("Expected count 2, got %v", result["count"])
	}
	
	// 清除数据
	stats.Clear()
	
	// 验证已清空
	clearedResult := stats.GetStats()
	if clearedResult["count"].(int64) != 0 {
		t.Errorf("Expected count 0 after clear, got %v", clearedResult["count"])
	}
}

func TestMemoryUsageStats(t *testing.T) {
	maxSize := int64(1024 * 1024) // 1MB
	stats := NewMemoryUsageStats(maxSize, 10)
	
	// 测试分配
	stats.RecordAllocation(100)
	stats.RecordAllocation(200)
	stats.RecordAllocation(300)
	
	result := stats.GetStats()
	
	// 验证当前大小
	if result["current_size_bytes"].(int64) != 600 {
		t.Errorf("Expected current size 600, got %v", result["current_size_bytes"])
	}
	
	// 验证峰值大小
	if result["peak_size_bytes"].(int64) != 600 {
		t.Errorf("Expected peak size 600, got %v", result["peak_size_bytes"])
	}
	
	// 验证分配计数
	if result["total_allocations"].(int64) != 3 {
		t.Errorf("Expected 3 allocations, got %v", result["total_allocations"])
	}
	
	// 测试释放
	stats.RecordDeallocation(150)
	
	resultAfterDealloc := stats.GetStats()
	
	// 验证当前大小减少
	if resultAfterDealloc["current_size_bytes"].(int64) != 450 {
		t.Errorf("Expected current size 450 after deallocation, got %v", resultAfterDealloc["current_size_bytes"])
	}
	
	// 验证峰值大小保持不变
	if resultAfterDealloc["peak_size_bytes"].(int64) != 600 {
		t.Errorf("Expected peak size to remain 600, got %v", resultAfterDealloc["peak_size_bytes"])
	}
	
	// 验证使用率计算
	expectedUsage := float64(450) / float64(maxSize) * 100
	actualUsage := resultAfterDealloc["usage_percent"].(float64)
	if actualUsage != expectedUsage {
		t.Errorf("Expected usage %.2f%%, got %.2f%%", expectedUsage, actualUsage)
	}
}

func TestMemoryUsageStatsNegativeProtection(t *testing.T) {
	stats := NewMemoryUsageStats(1024, 10)
	
	// 先分配一些内存
	stats.RecordAllocation(100)
	
	// 尝试释放超过已分配的内存
	stats.RecordDeallocation(200)
	
	result := stats.GetStats()
	
	// 当前大小不应该为负数
	if result["current_size_bytes"].(int64) != 0 {
		t.Errorf("Expected current size to be protected at 0, got %v", result["current_size_bytes"])
	}
}

func TestDetailedCacheMetrics(t *testing.T) {
	maxSize := int64(1024 * 1024)
	metrics := NewDetailedCacheMetrics(maxSize)
	
	// 测试初始状态
	initialStats := metrics.GetDetailedStats()
	if initialStats["hits"].(int64) != 0 {
		t.Errorf("Expected initial hits 0, got %v", initialStats["hits"])
	}
	
	// 模拟一些操作
	metrics.RecordGet("key1", true, 10*time.Millisecond, 100)   // 命中
	metrics.RecordGet("key2", false, 5*time.Millisecond, 0)     // 未命中
	metrics.RecordGet("key1", true, 15*time.Millisecond, 100)   // 再次命中
	
	metrics.RecordSet("key3", 20*time.Millisecond, 200, "string")
	metrics.RecordSet("key4", 25*time.Millisecond, 300, "[]byte")
	
	metrics.RecordDelete("key2", 5*time.Millisecond, 50)
	
	// 获取详细统计
	stats := metrics.GetDetailedStats()
	
	// 验证基础统计
	if stats["hits"].(int64) != 2 {
		t.Errorf("Expected 2 hits, got %v", stats["hits"])
	}
	
	if stats["misses"].(int64) != 1 {
		t.Errorf("Expected 1 miss, got %v", stats["misses"])
	}
	
	if stats["sets"].(int64) != 2 {
		t.Errorf("Expected 2 sets, got %v", stats["sets"])
	}
	
	if stats["deletes"].(int64) != 1 {
		t.Errorf("Expected 1 delete, got %v", stats["deletes"])
	}
	
	// 验证命中率
	expectedHitRate := float64(2) / float64(3) * 100
	actualHitRate := stats["hit_rate"].(float64)
	if actualHitRate != expectedHitRate {
		t.Errorf("Expected hit rate %.2f%%, got %.2f%%", expectedHitRate, actualHitRate)
	}
	
	// 验证响应时间统计存在
	if _, exists := stats["get_response_time"]; !exists {
		t.Error("Expected get_response_time stats to exist")
	}
	
	if _, exists := stats["set_response_time"]; !exists {
		t.Error("Expected set_response_time stats to exist")
	}
	
	// 验证类型分布
	typeStats := stats["type_distribution"].(map[string]int64)
	if typeStats["string"] != 1 {
		t.Errorf("Expected 1 string type, got %v", typeStats["string"])
	}
	
	if typeStats["[]byte"] != 1 {
		t.Errorf("Expected 1 []byte type, got %v", typeStats["[]byte"])
	}
	
	// 验证键访问频率
	topKeys := stats["top_accessed_keys"].([]map[string]interface{})
	if len(topKeys) < 1 {
		t.Error("Expected at least 1 top accessed key")
	}
	
	// key1应该是访问最频繁的（被访问了2次）
	if topKeys[0]["key"].(string) != "key1" || topKeys[0]["count"].(int64) != 2 {
		t.Errorf("Expected key1 with count 2 to be top accessed, got %v", topKeys[0])
	}
}

func TestDetailedCacheMetricsThresholds(t *testing.T) {
	metrics := NewDetailedCacheMetrics(1024)
	
	// 设置自定义阈值
	slowThreshold := 50 * time.Millisecond
	largeItemThreshold := int64(150)
	metrics.SetThresholds(slowThreshold, largeItemThreshold)
	
	// 记录一个慢操作
	metrics.RecordGet("slow_key", true, 100*time.Millisecond, 100)
	
	// 记录一个大项操作
	metrics.RecordSet("large_key", 10*time.Millisecond, 200, "large_item")
	
	stats := metrics.GetDetailedStats()
	
	// 验证慢操作计数
	if stats["slow_operation_count"].(int64) != 1 {
		t.Errorf("Expected 1 slow operation, got %v", stats["slow_operation_count"])
	}
	
	// 验证大项计数
	if stats["large_item_count"].(int64) != 1 {
		t.Errorf("Expected 1 large item, got %v", stats["large_item_count"])
	}
	
	// 验证阈值设置
	if stats["slow_operation_threshold_ms"].(float64) != 50.0 {
		t.Errorf("Expected slow threshold 50ms, got %v", stats["slow_operation_threshold_ms"])
	}
	
	if stats["large_item_threshold_bytes"].(int64) != 150 {
		t.Errorf("Expected large item threshold 150 bytes, got %v", stats["large_item_threshold_bytes"])
	}
}

func TestDetailedCacheMetricsError(t *testing.T) {
	metrics := NewDetailedCacheMetrics(1024)
	
	// 记录错误
	testError := fmt.Errorf("test error")
	metrics.RecordError(testError)
	
	stats := metrics.GetDetailedStats()
	
	// 验证错误计数
	if stats["error_count"].(int64) != 1 {
		t.Errorf("Expected 1 error, got %v", stats["error_count"])
	}
	
	// 验证错误信息
	if stats["last_error"].(string) != "test error" {
		t.Errorf("Expected 'test error', got %v", stats["last_error"])
	}
}

func TestDetailedCacheMetricsEviction(t *testing.T) {
	metrics := NewDetailedCacheMetrics(1024)
	
	// 记录淘汰操作
	metrics.RecordEviction(5, 500)
	
	stats := metrics.GetDetailedStats()
	
	// 验证淘汰计数
	if stats["evictions"].(int64) != 5 {
		t.Errorf("Expected 5 evictions, got %v", stats["evictions"])
	}
}

func TestDetailedCacheMetricsHealthStatus(t *testing.T) {
	metrics := NewDetailedCacheMetrics(1024)
	
	// 测试初始健康状态
	health := metrics.GetHealthStatus()
	
	if health["status"].(string) != "healthy" {
		t.Errorf("Expected healthy status, got %v", health["status"])
	}
	
	// 模拟低命中率场景
	for i := 0; i < 10; i++ {
		metrics.RecordGet(fmt.Sprintf("key%d", i), false, time.Millisecond, 0) // 全部未命中
	}
	
	healthAfterMisses := metrics.GetHealthStatus()
	
	// 命中率为0%，应该触发警告
	warnings := healthAfterMisses["warnings"].([]string)
	if len(warnings) == 0 {
		t.Error("Expected warnings for low hit rate")
	}
	
	// 验证状态变为degraded
	if healthAfterMisses["status"].(string) != "degraded" {
		t.Errorf("Expected degraded status for low hit rate, got %v", healthAfterMisses["status"])
	}
}

func TestDetailedCacheMetricsReset(t *testing.T) {
	metrics := NewDetailedCacheMetrics(1024)
	
	// 添加一些数据
	metrics.RecordGet("key1", true, 10*time.Millisecond, 100)
	metrics.RecordSet("key2", 20*time.Millisecond, 200, "string")
	metrics.RecordError(fmt.Errorf("test error"))
	
	// 验证有数据
	statsBefore := metrics.GetDetailedStats()
	if statsBefore["hits"].(int64) == 0 {
		t.Error("Expected some data before reset")
	}
	
	// 重置
	metrics.Reset()
	
	// 验证重置后
	statsAfter := metrics.GetDetailedStats()
	
	if statsAfter["hits"].(int64) != 0 {
		t.Errorf("Expected hits to be 0 after reset, got %v", statsAfter["hits"])
	}
	
	if statsAfter["sets"].(int64) != 0 {
		t.Errorf("Expected sets to be 0 after reset, got %v", statsAfter["sets"])
	}
	
	if statsAfter["error_count"].(int64) != 0 {
		t.Errorf("Expected error count to be 0 after reset, got %v", statsAfter["error_count"])
	}
}

func TestDetailedCacheMetricsPeriodicReporting(t *testing.T) {
	metrics := NewDetailedCacheMetrics(1024)
	
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	
	callbackCount := 0
	callback := func(stats map[string]interface{}) {
		callbackCount++
		if stats == nil {
			t.Error("Expected stats in callback")
		}
	}
	
	// 启动定期报告，每50ms一次
	go metrics.StartPeriodicReporting(ctx, 50*time.Millisecond, callback)
	
	// 等待几个周期
	time.Sleep(150 * time.Millisecond)
	
	// 验证回调被调用了多次
	if callbackCount < 2 {
		t.Errorf("Expected at least 2 callback calls, got %d", callbackCount)
	}
}

func TestDetailedCacheMetricsSizeDistribution(t *testing.T) {
	metrics := NewDetailedCacheMetrics(1024)
	
	// 记录不同大小的项
	metrics.RecordSet("small", 10*time.Millisecond, 500, "string")        // <=1KB
	metrics.RecordSet("medium", 10*time.Millisecond, 5000, "string")      // <=10KB
	metrics.RecordSet("large", 10*time.Millisecond, 50000, "string")      // <=100KB
	metrics.RecordSet("xlarge", 10*time.Millisecond, 2000000, "string")   // <=10MB
	
	stats := metrics.GetDetailedStats()
	sizeDistribution := stats["size_distribution"].(map[string]int64)
	
	if sizeDistribution["<=1KB"] != 1 {
		t.Errorf("Expected 1 item <=1KB, got %v", sizeDistribution["<=1KB"])
	}
	
	if sizeDistribution["<=10KB"] != 1 {
		t.Errorf("Expected 1 item <=10KB, got %v", sizeDistribution["<=10KB"])
	}
	
	if sizeDistribution["<=100KB"] != 1 {
		t.Errorf("Expected 1 item <=100KB, got %v", sizeDistribution["<=100KB"])
	}
	
	if sizeDistribution["<=10MB"] != 1 {
		t.Errorf("Expected 1 item <=10MB, got %v", sizeDistribution["<=10MB"])
	}
}

func TestDetailedCacheMetricsHourlyStats(t *testing.T) {
	metrics := NewDetailedCacheMetrics(1024)
	
	// 记录一些命中
	for i := 0; i < 5; i++ {
		metrics.RecordGet("key", true, time.Millisecond, 100)
	}
	
	stats := metrics.GetDetailedStats()
	hourlyStats := stats["hourly_hits"].(map[string]int64)
	
	// 当前小时应该有5次命中
	currentHour := fmt.Sprintf("%d", time.Now().Hour())
	if hourlyStats[currentHour] != 5 {
		t.Errorf("Expected 5 hits in current hour, got %v", hourlyStats[currentHour])
	}
}

// 基准测试
func BenchmarkDetailedCacheMetricsRecordGet(b *testing.B) {
	metrics := NewDetailedCacheMetrics(1024)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics.RecordGet("test_key", true, time.Microsecond, 100)
	}
}

func BenchmarkDetailedCacheMetricsRecordSet(b *testing.B) {
	metrics := NewDetailedCacheMetrics(1024)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics.RecordSet("test_key", time.Microsecond, 100, "string")
	}
}

func BenchmarkDetailedCacheMetricsGetStats(b *testing.B) {
	metrics := NewDetailedCacheMetrics(1024)
	
	// 添加一些数据
	for i := 0; i < 1000; i++ {
		metrics.RecordGet(fmt.Sprintf("key%d", i), i%2 == 0, time.Microsecond, 100)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = metrics.GetDetailedStats()
	}
}

func BenchmarkResponseTimeStatsRecord(b *testing.B) {
	stats := NewResponseTimeStats(1000)
	duration := 10 * time.Millisecond
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats.Record(duration)
	}
}

func BenchmarkResponseTimeStatsGetStats(b *testing.B) {
	stats := NewResponseTimeStats(1000)
	
	// 添加一些样本
	for i := 0; i < 1000; i++ {
		stats.Record(time.Duration(i) * time.Microsecond)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = stats.GetStats()
	}
}