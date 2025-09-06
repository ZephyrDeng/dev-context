package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDefaultMonitoringConfig(t *testing.T) {
	config := DefaultMonitoringConfig()
	
	if config.HTTPPort != 8080 {
		t.Errorf("Expected default HTTP port 8080, got %d", config.HTTPPort)
	}
	
	if config.LoggingEnabled != true {
		t.Error("Expected logging to be enabled by default")
	}
	
	if config.LogLevel != "info" {
		t.Errorf("Expected default log level 'info', got %s", config.LogLevel)
	}
	
	if config.AlertThresholds == nil {
		t.Error("Expected alert thresholds to be set")
	}
	
	if config.AlertThresholds.LowHitRatePercent != 20.0 {
		t.Errorf("Expected low hit rate threshold 20.0, got %f", config.AlertThresholds.LowHitRatePercent)
	}
}

func TestCacheMonitorCreation(t *testing.T) {
	// 创建测试用的缓存管理器和指标
	config := DefaultCacheConfig()
	cm := NewCacheManager(config)
	defer cm.Close()
	
	metrics := NewDetailedCacheMetrics(config.MaxSize)
	monitoringConfig := DefaultMonitoringConfig()
	monitoringConfig.EnableHTTPEndpoints = false // 测试时禁用HTTP服务器
	
	monitor := NewCacheMonitor(cm, metrics, monitoringConfig)
	
	if monitor.cacheManager != cm {
		t.Error("Cache manager not set correctly")
	}
	
	if monitor.metrics != metrics {
		t.Error("Metrics not set correctly")
	}
	
	if monitor.config != monitoringConfig {
		t.Error("Config not set correctly")
	}
	
	if monitor.isRunning {
		t.Error("Monitor should not be running initially")
	}
}

func TestCacheMonitorStartStop(t *testing.T) {
	// 创建测试环境
	config := DefaultCacheConfig()
	cm := NewCacheManager(config)
	defer cm.Close()
	
	metrics := NewDetailedCacheMetrics(config.MaxSize)
	monitoringConfig := DefaultMonitoringConfig()
	monitoringConfig.EnableHTTPEndpoints = false
	monitoringConfig.MetricsInterval = 50 * time.Millisecond
	monitoringConfig.HealthCheckInterval = 50 * time.Millisecond
	
	monitor := NewCacheMonitor(cm, metrics, monitoringConfig)
	
	// 启动监控
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	
	err := monitor.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start monitor: %v", err)
	}
	
	if !monitor.isRunning {
		t.Error("Monitor should be running after start")
	}
	
	// 等待一短时间让后台协程运行
	time.Sleep(100 * time.Millisecond)
	
	// 停止监控
	err = monitor.Stop()
	if err != nil {
		t.Fatalf("Failed to stop monitor: %v", err)
	}
	
	if monitor.isRunning {
		t.Error("Monitor should not be running after stop")
	}
}

func TestCacheMonitorDuplicateStart(t *testing.T) {
	config := DefaultCacheConfig()
	cm := NewCacheManager(config)
	defer cm.Close()
	
	metrics := NewDetailedCacheMetrics(config.MaxSize)
	monitoringConfig := DefaultMonitoringConfig()
	monitoringConfig.EnableHTTPEndpoints = false
	
	monitor := NewCacheMonitor(cm, metrics, monitoringConfig)
	
	ctx := context.Background()
	
	// 第一次启动
	err := monitor.Start(ctx)
	if err != nil {
		t.Fatalf("First start failed: %v", err)
	}
	defer monitor.Stop()
	
	// 第二次启动应该失败
	err = monitor.Start(ctx)
	if err == nil {
		t.Error("Expected error when starting already running monitor")
	}
}

func TestCacheMonitorHTTPEndpoints(t *testing.T) {
	config := DefaultCacheConfig()
	cm := NewCacheManager(config)
	defer cm.Close()
	
	metrics := NewDetailedCacheMetrics(config.MaxSize)
	monitoringConfig := DefaultMonitoringConfig()
	monitoringConfig.EnableHTTPEndpoints = true
	monitoringConfig.HTTPPort = 0 // 让系统分配端口
	
	monitor := NewCacheMonitor(cm, metrics, monitoringConfig)
	monitor.setupHTTPServer()
	
	// 测试健康检查端点
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	monitor.handleHealth(rr, req)
	
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Health endpoint returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	
	var healthResponse map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &healthResponse)
	if err != nil {
		t.Fatalf("Failed to parse health response: %v", err)
	}
	
	if _, exists := healthResponse["status"]; !exists {
		t.Error("Health response missing status field")
	}
}

func TestCacheMonitorHTTPMetricsEndpoint(t *testing.T) {
	config := DefaultCacheConfig()
	cm := NewCacheManager(config)
	defer cm.Close()
	
	metrics := NewDetailedCacheMetrics(config.MaxSize)
	monitor := NewCacheMonitor(cm, metrics, nil)
	
	// 添加一些测试数据
	metrics.RecordGet("test_key", true, 10*time.Millisecond, 100)
	metrics.RecordSet("test_key", 5*time.Millisecond, 100, "string")
	
	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()
	monitor.handleMetrics(rr, req)
	
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Metrics endpoint returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	
	var metricsResponse map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &metricsResponse)
	if err != nil {
		t.Fatalf("Failed to parse metrics response: %v", err)
	}
	
	if _, exists := metricsResponse["hits"]; !exists {
		t.Error("Metrics response missing hits field")
	}
	
	if _, exists := metricsResponse["sets"]; !exists {
		t.Error("Metrics response missing sets field")
	}
}

func TestCacheMonitorHTTPKeysEndpoint(t *testing.T) {
	config := DefaultCacheConfig()
	cm := NewCacheManager(config)
	defer cm.Close()
	
	// 添加一些缓存项
	cm.Set("key1", "value1")
	cm.Set("key2", "value2")
	cm.Set("test_key", "test_value")
	
	metrics := NewDetailedCacheMetrics(config.MaxSize)
	monitor := NewCacheMonitor(cm, metrics, nil)
	
	// 测试获取所有键
	req := httptest.NewRequest("GET", "/keys", nil)
	rr := httptest.NewRecorder()
	monitor.handleKeys(rr, req)
	
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Keys endpoint returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	
	var keysResponse map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &keysResponse)
	if err != nil {
		t.Fatalf("Failed to parse keys response: %v", err)
	}
	
	keys, exists := keysResponse["keys"].([]interface{})
	if !exists {
		t.Error("Keys response missing keys field")
	}
	
	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}
	
	// 测试过滤键
	req = httptest.NewRequest("GET", "/keys?pattern=test", nil)
	rr = httptest.NewRecorder()
	monitor.handleKeys(rr, req)
	
	err = json.Unmarshal(rr.Body.Bytes(), &keysResponse)
	if err != nil {
		t.Fatalf("Failed to parse filtered keys response: %v", err)
	}
	
	filteredKeys, _ := keysResponse["keys"].([]interface{})
	if len(filteredKeys) != 1 {
		t.Errorf("Expected 1 filtered key, got %d", len(filteredKeys))
	}
	
	// 测试限制数量
	req = httptest.NewRequest("GET", "/keys?limit=2", nil)
	rr = httptest.NewRecorder()
	monitor.handleKeys(rr, req)
	
	err = json.Unmarshal(rr.Body.Bytes(), &keysResponse)
	if err != nil {
		t.Fatalf("Failed to parse limited keys response: %v", err)
	}
	
	limitedKeys, _ := keysResponse["keys"].([]interface{})
	if len(limitedKeys) != 2 {
		t.Errorf("Expected 2 limited keys, got %d", len(limitedKeys))
	}
}

func TestCacheMonitorHTTPKeyDetailsEndpoint(t *testing.T) {
	config := DefaultCacheConfig()
	cm := NewCacheManager(config)
	defer cm.Close()
	
	// 添加一个缓存项
	cm.Set("test_key", "test_value")
	
	metrics := NewDetailedCacheMetrics(config.MaxSize)
	monitor := NewCacheMonitor(cm, metrics, nil)
	
	// 测试获取存在的键详情
	req := httptest.NewRequest("GET", "/keys/test_key", nil)
	rr := httptest.NewRecorder()
	monitor.handleKeyDetails(rr, req)
	
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Key details endpoint returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	
	var detailsResponse map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &detailsResponse)
	if err != nil {
		t.Fatalf("Failed to parse key details response: %v", err)
	}
	
	if detailsResponse["key"].(string) != "test_key" {
		t.Errorf("Expected key 'test_key', got %v", detailsResponse["key"])
	}
	
	if detailsResponse["data"].(string) != "test_value" {
		t.Errorf("Expected data 'test_value', got %v", detailsResponse["data"])
	}
	
	// 测试获取不存在的键
	req = httptest.NewRequest("GET", "/keys/nonexistent_key", nil)
	rr = httptest.NewRecorder()
	monitor.handleKeyDetails(rr, req)
	
	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("Expected 404 for nonexistent key, got %v", status)
	}
}

func TestCacheMonitorHTTPClearEndpoint(t *testing.T) {
	config := DefaultCacheConfig()
	cm := NewCacheManager(config)
	defer cm.Close()
	
	// 添加一些缓存项
	cm.Set("key1", "value1")
	cm.Set("key2", "value2")
	
	metrics := NewDetailedCacheMetrics(config.MaxSize)
	monitor := NewCacheMonitor(cm, metrics, nil)
	
	// 验证缓存不为空
	if cm.Size() == 0 {
		t.Error("Cache should not be empty initially")
	}
	
	// 测试清空所有缓存
	req := httptest.NewRequest("POST", "/clear?action=all", nil)
	rr := httptest.NewRecorder()
	monitor.handleClear(rr, req)
	
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Clear endpoint returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	
	// 验证缓存已清空
	if cm.Size() != 0 {
		t.Errorf("Expected cache size 0 after clear, got %d", cm.Size())
	}
	
	// 测试无效的操作
	req = httptest.NewRequest("POST", "/clear?action=invalid", nil)
	rr = httptest.NewRecorder()
	monitor.handleClear(rr, req)
	
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid action, got %v", status)
	}
}

func TestCacheMonitorHTTPGCEndpoint(t *testing.T) {
	config := DefaultCacheConfig()
	cm := NewCacheManager(config)
	defer cm.Close()
	
	metrics := NewDetailedCacheMetrics(config.MaxSize)
	monitor := NewCacheMonitor(cm, metrics, nil)
	
	req := httptest.NewRequest("POST", "/gc", nil)
	rr := httptest.NewRecorder()
	monitor.handleGC(rr, req)
	
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("GC endpoint returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	
	var gcResponse map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &gcResponse)
	if err != nil {
		t.Fatalf("Failed to parse GC response: %v", err)
	}
	
	if _, exists := gcResponse["heap_before"]; !exists {
		t.Error("GC response missing heap_before field")
	}
	
	if _, exists := gcResponse["heap_after"]; !exists {
		t.Error("GC response missing heap_after field")
	}
}

func TestCacheMonitorHTTPMethodNotAllowed(t *testing.T) {
	config := DefaultCacheConfig()
	cm := NewCacheManager(config)
	defer cm.Close()
	
	metrics := NewDetailedCacheMetrics(config.MaxSize)
	monitor := NewCacheMonitor(cm, metrics, nil)
	
	// 测试GET端点的POST请求
	req := httptest.NewRequest("POST", "/health", nil)
	rr := httptest.NewRecorder()
	monitor.handleHealth(rr, req)
	
	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for wrong method, got %v", status)
	}
	
	// 测试POST端点的GET请求
	req = httptest.NewRequest("GET", "/clear", nil)
	rr = httptest.NewRecorder()
	monitor.handleClear(rr, req)
	
	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for wrong method, got %v", status)
	}
}

func TestCacheMonitorAlerts(t *testing.T) {
	config := DefaultCacheConfig()
	cm := NewCacheManager(config)
	defer cm.Close()
	
	metrics := NewDetailedCacheMetrics(config.MaxSize)
	monitoringConfig := DefaultMonitoringConfig()
	monitoringConfig.EnableHTTPEndpoints = false
	
	monitor := NewCacheMonitor(cm, metrics, monitoringConfig)
	
	// 添加告警回调
	alertReceived := false
	var receivedAlert *Alert
	callback := func(alert *Alert) {
		alertReceived = true
		receivedAlert = alert
	}
	monitor.AddAlertCallback(callback)
	
	// 触发低命中率告警
	stats := map[string]interface{}{
		"hit_rate":                10.0, // 低于阈值20%
		"memory_usage_percent":    50.0,
		"error_count":             int64(5),
		"slow_operation_count":    int64(10),
	}
	
	monitor.checkAlerts(stats)
	
	// 等待异步回调
	time.Sleep(10 * time.Millisecond)
	
	if !alertReceived {
		t.Error("Expected alert to be received")
	}
	
	if receivedAlert == nil {
		t.Error("Expected alert object to be set")
	}
	
	if receivedAlert != nil && receivedAlert.Level != "warning" {
		t.Errorf("Expected warning level alert, got %s", receivedAlert.Level)
	}
	
	if receivedAlert != nil && !strings.Contains(receivedAlert.Message, "命中率过低") {
		t.Errorf("Expected hit rate alert message, got %s", receivedAlert.Message)
	}
}

func TestCacheMonitorAlertDeduplication(t *testing.T) {
	config := DefaultCacheConfig()
	cm := NewCacheManager(config)
	defer cm.Close()
	
	metrics := NewDetailedCacheMetrics(config.MaxSize)
	monitoringConfig := DefaultMonitoringConfig()
	
	monitor := NewCacheMonitor(cm, metrics, monitoringConfig)
	
	alertCount := 0
	callback := func(alert *Alert) {
		alertCount++
	}
	monitor.AddAlertCallback(callback)
	
	// 连续记录相同的告警
	message := "测试告警"
	monitor.recordAlert("warning", message, nil)
	monitor.recordAlert("warning", message, nil)
	monitor.recordAlert("warning", message, nil)
	
	// 等待异步处理
	time.Sleep(10 * time.Millisecond)
	
	// 由于去重机制，应该只收到一个告警
	if alertCount != 1 {
		t.Errorf("Expected 1 alert due to deduplication, got %d", alertCount)
	}
}

func TestCacheMonitorDebugInfo(t *testing.T) {
	config := DefaultCacheConfig()
	cm := NewCacheManager(config)
	defer cm.Close()
	
	metrics := NewDetailedCacheMetrics(config.MaxSize)
	monitor := NewCacheMonitor(cm, metrics, nil)
	
	debugInfo := monitor.GetDebugInfo()
	
	if debugInfo == nil {
		t.Error("Expected debug info to be returned")
	}
	
	if _, exists := debugInfo["monitor_status"]; !exists {
		t.Error("Debug info missing monitor_status")
	}
	
	if _, exists := debugInfo["system_memory"]; !exists {
		t.Error("Debug info missing system_memory")
	}
	
	if _, exists := debugInfo["cache_details"]; !exists {
		t.Error("Debug info missing cache_details")
	}
	
	if _, exists := debugInfo["internal_state"]; !exists {
		t.Error("Debug info missing internal_state")
	}
}

func TestCacheMonitorConfigurationManagement(t *testing.T) {
	config := DefaultCacheConfig()
	cm := NewCacheManager(config)
	defer cm.Close()
	
	metrics := NewDetailedCacheMetrics(config.MaxSize)
	originalConfig := DefaultMonitoringConfig()
	
	monitor := NewCacheMonitor(cm, metrics, originalConfig)
	
	// 获取配置
	retrievedConfig := monitor.GetConfiguration()
	if retrievedConfig.HTTPPort != originalConfig.HTTPPort {
		t.Errorf("Expected HTTP port %d, got %d", originalConfig.HTTPPort, retrievedConfig.HTTPPort)
	}
	
	// 更新配置
	newConfig := DefaultMonitoringConfig()
	newConfig.HTTPPort = 9090
	newConfig.LogLevel = "debug"
	
	err := monitor.UpdateConfiguration(newConfig)
	if err != nil {
		t.Fatalf("Failed to update configuration: %v", err)
	}
	
	// 验证配置已更新
	updatedConfig := monitor.GetConfiguration()
	if updatedConfig.HTTPPort != 9090 {
		t.Errorf("Expected updated HTTP port 9090, got %d", updatedConfig.HTTPPort)
	}
	
	if updatedConfig.LogLevel != "debug" {
		t.Errorf("Expected updated log level 'debug', got %s", updatedConfig.LogLevel)
	}
	
	// 测试nil配置
	err = monitor.UpdateConfiguration(nil)
	if err == nil {
		t.Error("Expected error when updating with nil config")
	}
}

func TestCacheMonitoringStats(t *testing.T) {
	config := DefaultCacheConfig()
	cm := NewCacheManager(config)
	defer cm.Close()
	
	metrics := NewDetailedCacheMetrics(config.MaxSize)
	monitoringConfig := DefaultMonitoringConfig()
	
	monitor := NewCacheMonitor(cm, metrics, monitoringConfig)
	
	// 添加一些回调
	monitor.AddAlertCallback(func(alert *Alert) {})
	monitor.AddAlertCallback(func(alert *Alert) {})
	
	stats := monitor.GetMonitoringStats()
	
	if stats["monitor_running"].(bool) != false {
		t.Error("Expected monitor to not be running initially")
	}
	
	if stats["http_server_enabled"].(bool) != monitoringConfig.EnableHTTPEndpoints {
		t.Error("HTTP server enabled status mismatch")
	}
	
	if stats["alert_callbacks_count"].(int) != 2 {
		t.Errorf("Expected 2 alert callbacks, got %v", stats["alert_callbacks_count"])
	}
}

func TestCacheMonitorSimulateLoad(t *testing.T) {
	config := DefaultCacheConfig()
	cm := NewCacheManager(config)
	defer cm.Close()
	
	metrics := NewDetailedCacheMetrics(config.MaxSize)
	monitor := NewCacheMonitor(cm, metrics, nil)
	
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	
	// 模拟负载：每秒10次操作
	go monitor.SimulateLoad(ctx, 200*time.Millisecond, 10)
	
	// 等待负载执行
	time.Sleep(250 * time.Millisecond)
	
	// 验证缓存中有一些数据
	if cm.Size() == 0 {
		t.Error("Expected some cache entries after load simulation")
	}
	
	// 验证指标记录了一些操作
	stats := metrics.GetDetailedStats()
	totalOps := stats["hits"].(int64) + stats["misses"].(int64) + stats["sets"].(int64) + stats["deletes"].(int64)
	
	if totalOps == 0 {
		t.Error("Expected some operations to be recorded after load simulation")
	}
}

// 基准测试
func BenchmarkCacheMonitorRecordAlert(b *testing.B) {
	config := DefaultCacheConfig()
	cm := NewCacheManager(config)
	defer cm.Close()
	
	metrics := NewDetailedCacheMetrics(config.MaxSize)
	monitor := NewCacheMonitor(cm, metrics, nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.recordAlert("info", fmt.Sprintf("test alert %d", i), nil)
	}
}

func BenchmarkCacheMonitorGetDebugInfo(b *testing.B) {
	config := DefaultCacheConfig()
	cm := NewCacheManager(config)
	defer cm.Close()
	
	metrics := NewDetailedCacheMetrics(config.MaxSize)
	monitor := NewCacheMonitor(cm, metrics, nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = monitor.GetDebugInfo()
	}
}

func BenchmarkCacheMonitorCheckAlerts(b *testing.B) {
	config := DefaultCacheConfig()
	cm := NewCacheManager(config)
	defer cm.Close()
	
	metrics := NewDetailedCacheMetrics(config.MaxSize)
	monitor := NewCacheMonitor(cm, metrics, nil)
	
	stats := map[string]interface{}{
		"hit_rate":                50.0,
		"memory_usage_percent":    75.0,
		"error_count":             int64(10),
		"slow_operation_count":    int64(5),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.checkAlerts(stats)
	}
}