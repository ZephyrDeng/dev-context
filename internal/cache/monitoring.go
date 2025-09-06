package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	EnableHTTPEndpoints  bool          `json:"enable_http_endpoints"`
	HTTPPort            int           `json:"http_port"`
	LoggingEnabled      bool          `json:"logging_enabled"`
	LogLevel            string        `json:"log_level"` // debug, info, warn, error
	MetricsInterval     time.Duration `json:"metrics_interval"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	AlertThresholds     *AlertThresholds `json:"alert_thresholds"`
}

// AlertThresholds 告警阈值配置
type AlertThresholds struct {
	LowHitRatePercent        float64       `json:"low_hit_rate_percent"`
	HighMemoryUsagePercent   float64       `json:"high_memory_usage_percent"`
	MaxResponseTimeMs        float64       `json:"max_response_time_ms"`
	MaxErrorCount            int64         `json:"max_error_count"`
	MaxSlowOperations        int64         `json:"max_slow_operations"`
	HealthCheckTimeout       time.Duration `json:"health_check_timeout"`
}

// DefaultMonitoringConfig 默认监控配置
func DefaultMonitoringConfig() *MonitoringConfig {
	return &MonitoringConfig{
		EnableHTTPEndpoints:  false,
		HTTPPort:            8080,
		LoggingEnabled:      true,
		LogLevel:            "info",
		MetricsInterval:     1 * time.Minute,
		HealthCheckInterval: 30 * time.Second,
		AlertThresholds: &AlertThresholds{
			LowHitRatePercent:        20.0,
			HighMemoryUsagePercent:   90.0,
			MaxResponseTimeMs:        1000.0,
			MaxErrorCount:            100,
			MaxSlowOperations:        50,
			HealthCheckTimeout:       5 * time.Second,
		},
	}
}

// AlertCallback 告警回调函数类型
type AlertCallback func(alert *Alert)

// Alert 告警信息
type Alert struct {
	Level     string                 `json:"level"`      // info, warning, error, critical
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// CacheMonitor 缓存监控器
type CacheMonitor struct {
	mu            sync.RWMutex
	cacheManager  *CacheManager
	metrics       *DetailedCacheMetrics
	config        *MonitoringConfig
	
	// HTTP服务器
	httpServer    *http.Server
	serverMux     *http.ServeMux
	
	// 告警系统
	alertCallbacks []AlertCallback
	lastAlerts     map[string]time.Time
	
	// 监控状态
	isRunning     bool
	stopChan      chan struct{}
	wg            sync.WaitGroup
	
	// 调试信息
	debugData     map[string]interface{}
}

// NewCacheMonitor 创建缓存监控器
func NewCacheMonitor(cm *CacheManager, metrics *DetailedCacheMetrics, config *MonitoringConfig) *CacheMonitor {
	if config == nil {
		config = DefaultMonitoringConfig()
	}
	
	monitor := &CacheMonitor{
		cacheManager:   cm,
		metrics:        metrics,
		config:         config,
		alertCallbacks: make([]AlertCallback, 0),
		lastAlerts:     make(map[string]time.Time),
		stopChan:       make(chan struct{}),
		debugData:      make(map[string]interface{}),
	}
	
	if config.EnableHTTPEndpoints {
		monitor.setupHTTPServer()
	}
	
	return monitor
}

// Start 启动监控
func (m *CacheMonitor) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.isRunning {
		return fmt.Errorf("monitor is already running")
	}
	
	m.isRunning = true
	
	// 启动HTTP服务器
	if m.config.EnableHTTPEndpoints && m.httpServer != nil {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			
			if err := m.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				m.recordAlert("error", fmt.Sprintf("HTTP监控服务器错误: %v", err), nil)
			}
		}()
	}
	
	// 启动指标收集
	if m.config.MetricsInterval > 0 {
		m.wg.Add(1)
		go m.metricsCollector(ctx)
	}
	
	// 启动健康检查
	if m.config.HealthCheckInterval > 0 {
		m.wg.Add(1)
		go m.healthChecker(ctx)
	}
	
	return nil
}

// Stop 停止监控
func (m *CacheMonitor) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !m.isRunning {
		return nil
	}
	
	m.isRunning = false
	close(m.stopChan)
	
	// 停止HTTP服务器
	if m.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.httpServer.Shutdown(ctx)
	}
	
	// 等待所有协程结束
	m.wg.Wait()
	
	return nil
}

// setupHTTPServer 设置HTTP监控服务器
func (m *CacheMonitor) setupHTTPServer() {
	m.serverMux = http.NewServeMux()
	
	// 注册监控端点
	m.serverMux.HandleFunc("/health", m.handleHealth)
	m.serverMux.HandleFunc("/metrics", m.handleMetrics)
	m.serverMux.HandleFunc("/stats", m.handleStats)
	m.serverMux.HandleFunc("/debug", m.handleDebug)
	m.serverMux.HandleFunc("/config", m.handleConfig)
	m.serverMux.HandleFunc("/keys", m.handleKeys)
	m.serverMux.HandleFunc("/keys/", m.handleKeyDetails)
	m.serverMux.HandleFunc("/clear", m.handleClear)
	m.serverMux.HandleFunc("/gc", m.handleGC)
	
	m.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", m.config.HTTPPort),
		Handler: m.serverMux,
		
		// 基础安全配置
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

// 处理健康检查端点
func (m *CacheMonitor) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	health := m.metrics.GetHealthStatus()
	
	w.Header().Set("Content-Type", "application/json")
	
	// 根据健康状态设置HTTP状态码
	status := health["status"].(string)
	switch status {
	case "healthy":
		w.WriteHeader(http.StatusOK)
	case "degraded":
		w.WriteHeader(http.StatusOK) // 仍然可用
	case "critical":
		w.WriteHeader(http.StatusServiceUnavailable)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
	
	json.NewEncoder(w).Encode(health)
}

// 处理指标端点
func (m *CacheMonitor) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	stats := m.metrics.GetDetailedStats()
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	json.NewEncoder(w).Encode(stats)
}

// 处理统计信息端点
func (m *CacheMonitor) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// 合并所有统计信息
	allStats := make(map[string]interface{})
	
	// 缓存统计
	cacheStats := m.cacheManager.GetStats()
	for k, v := range cacheStats {
		allStats[k] = v
	}
	
	// 系统统计
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	allStats["system"] = map[string]interface{}{
		"goroutines":       runtime.NumGoroutine(),
		"heap_alloc":       memStats.HeapAlloc,
		"heap_sys":         memStats.HeapSys,
		"heap_idle":        memStats.HeapIdle,
		"heap_inuse":       memStats.HeapInuse,
		"gc_cycles":        memStats.NumGC,
		"last_gc":          time.Unix(0, int64(memStats.LastGC)),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	json.NewEncoder(w).Encode(allStats)
}

// 处理调试信息端点
func (m *CacheMonitor) handleDebug(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	debug := m.GetDebugInfo()
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	json.NewEncoder(w).Encode(debug)
}

// 处理配置端点
func (m *CacheMonitor) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(m.config)
		
	case http.MethodPost:
		// 更新配置（简化版，实际应用中需要更严格的验证）
		var newConfig MonitoringConfig
		if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		
		m.mu.Lock()
		m.config = &newConfig
		m.mu.Unlock()
		
		w.WriteHeader(http.StatusOK)
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// 处理键列表端点
func (m *CacheMonitor) handleKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// 获取查询参数
	query := r.URL.Query()
	limitStr := query.Get("limit")
	pattern := query.Get("pattern")
	
	keys := m.cacheManager.GetKeys()
	
	// 过滤键
	if pattern != "" {
		filteredKeys := make([]string, 0)
		for _, key := range keys {
			if strings.Contains(key, pattern) {
				filteredKeys = append(filteredKeys, key)
			}
		}
		keys = filteredKeys
	}
	
	// 限制数量
	if limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit < len(keys) {
			keys = keys[:limit]
		}
	}
	
	result := map[string]interface{}{
		"keys":  keys,
		"total": len(keys),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	json.NewEncoder(w).Encode(result)
}

// 处理单个键详情端点
func (m *CacheMonitor) handleKeyDetails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// 从URL路径中提取键名
	key := strings.TrimPrefix(r.URL.Path, "/keys/")
	if key == "" {
		http.Error(w, "Key not specified", http.StatusBadRequest)
		return
	}
	
	// 尝试获取缓存项
	if data, found := m.cacheManager.Get(key); found {
		// 获取存储中的详细信息
		if result, exists := m.cacheManager.storage.Get(key); exists {
			details := map[string]interface{}{
				"key":           key,
				"data":          data,
				"timestamp":     result.Timestamp,
				"expiry":        result.Expiry,
				"access_count":  result.AccessCount,
				"size":          result.EstimateSize(),
				"ttl_seconds":   int(time.Until(result.Expiry).Seconds()),
				"is_expired":    result.IsExpired(),
			}
			
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(details)
		} else {
			http.Error(w, "Key found but details unavailable", http.StatusInternalServerError)
		}
	} else {
		http.Error(w, "Key not found", http.StatusNotFound)
	}
}

// 处理清理端点
func (m *CacheMonitor) handleClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	query := r.URL.Query()
	action := query.Get("action")
	
	switch action {
	case "all":
		m.cacheManager.Clear()
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "All cache cleared")
		
	case "expired":
		m.cacheManager.ForceCleanup()
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Expired items cleared")
		
	default:
		http.Error(w, "Invalid action. Use 'all' or 'expired'", http.StatusBadRequest)
	}
}

// 处理垃圾回收端点
func (m *CacheMonitor) handleGC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var memStatsBefore, memStatsAfter runtime.MemStats
	runtime.ReadMemStats(&memStatsBefore)
	
	runtime.GC()
	runtime.ReadMemStats(&memStatsAfter)
	
	result := map[string]interface{}{
		"heap_before": memStatsBefore.HeapAlloc,
		"heap_after":  memStatsAfter.HeapAlloc,
		"freed":       int64(memStatsBefore.HeapAlloc) - int64(memStatsAfter.HeapAlloc),
		"timestamp":   time.Now(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	json.NewEncoder(w).Encode(result)
}

// metricsCollector 指标收集协程
func (m *CacheMonitor) metricsCollector(ctx context.Context) {
	defer m.wg.Done()
	
	ticker := time.NewTicker(m.config.MetricsInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.collectMetrics()
		}
	}
}

// healthChecker 健康检查协程
func (m *CacheMonitor) healthChecker(ctx context.Context) {
	defer m.wg.Done()
	
	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.performHealthCheck()
		}
	}
}

// collectMetrics 收集指标
func (m *CacheMonitor) collectMetrics() {
	stats := m.metrics.GetDetailedStats()
	
	// 检查告警条件
	m.checkAlerts(stats)
	
	// 记录调试信息
	m.mu.Lock()
	m.debugData["last_collection"] = time.Now()
	m.debugData["collection_count"] = m.debugData["collection_count"].(int64) + 1
	m.mu.Unlock()
}

// performHealthCheck 执行健康检查
func (m *CacheMonitor) performHealthCheck() {
	health := m.metrics.GetHealthStatus()
	
	if health["status"].(string) != "healthy" {
		metadata := map[string]interface{}{
			"health_status": health,
		}
		m.recordAlert("warning", "缓存健康状态异常", metadata)
	}
}

// checkAlerts 检查告警条件
func (m *CacheMonitor) checkAlerts(stats map[string]interface{}) {
	thresholds := m.config.AlertThresholds
	
	// 检查命中率
	if hitRate, ok := stats["hit_rate"].(float64); ok && hitRate < thresholds.LowHitRatePercent {
		m.recordAlert("warning", fmt.Sprintf("缓存命中率过低: %.2f%%", hitRate), nil)
	}
	
	// 检查内存使用
	if memUsage, ok := stats["memory_usage_percent"].(float64); ok && memUsage > thresholds.HighMemoryUsagePercent {
		level := "warning"
		if memUsage > 95 {
			level = "critical"
		}
		m.recordAlert(level, fmt.Sprintf("内存使用率过高: %.2f%%", memUsage), nil)
	}
	
	// 检查错误数量
	if errorCount, ok := stats["error_count"].(int64); ok && errorCount > thresholds.MaxErrorCount {
		m.recordAlert("error", fmt.Sprintf("错误数量过多: %d", errorCount), nil)
	}
	
	// 检查慢操作
	if slowOps, ok := stats["slow_operation_count"].(int64); ok && slowOps > thresholds.MaxSlowOperations {
		m.recordAlert("warning", fmt.Sprintf("慢操作数量过多: %d", slowOps), nil)
	}
}

// recordAlert 记录告警
func (m *CacheMonitor) recordAlert(level, message string, metadata map[string]interface{}) {
	alert := &Alert{
		Level:     level,
		Message:   message,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}
	
	// 检查告警去重（同样的消息在5分钟内只告警一次）
	alertKey := fmt.Sprintf("%s:%s", level, message)
	m.mu.RLock()
	lastTime, exists := m.lastAlerts[alertKey]
	m.mu.RUnlock()
	
	if exists && time.Since(lastTime) < 5*time.Minute {
		return // 跳过重复告警
	}
	
	m.mu.Lock()
	m.lastAlerts[alertKey] = time.Now()
	m.mu.Unlock()
	
	// 调用告警回调
	for _, callback := range m.alertCallbacks {
		go callback(alert) // 异步调用避免阻塞
	}
}

// AddAlertCallback 添加告警回调
func (m *CacheMonitor) AddAlertCallback(callback AlertCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.alertCallbacks = append(m.alertCallbacks, callback)
}

// GetDebugInfo 获取调试信息
func (m *CacheMonitor) GetDebugInfo() map[string]interface{} {
	debug := make(map[string]interface{})
	
	// 基础信息
	debug["monitor_status"] = map[string]interface{}{
		"running":         m.isRunning,
		"goroutines":      runtime.NumGoroutine(),
		"config":          m.config,
	}
	
	// 系统信息
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	debug["system_memory"] = map[string]interface{}{
		"heap_alloc":    memStats.HeapAlloc,
		"heap_sys":      memStats.HeapSys,
		"heap_objects":  memStats.HeapObjects,
		"gc_cycles":     memStats.NumGC,
		"last_gc":       time.Unix(0, int64(memStats.LastGC)),
	}
	
	// 缓存详情
	debug["cache_details"] = map[string]interface{}{
		"size":                         m.cacheManager.Size(),
		"total_size":                   m.cacheManager.TotalSize(),
		"max_size":                     m.cacheManager.MaxSize(),
		"active_coalescing_groups":     m.cacheManager.ActiveCoalescingGroupsCount(),
	}
	
	// 内部状态
	m.mu.RLock()
	debug["internal_state"] = map[string]interface{}{
		"alert_callbacks_count": len(m.alertCallbacks),
		"last_alerts_count":     len(m.lastAlerts),
		"debug_data":            m.debugData,
	}
	m.mu.RUnlock()
	
	return debug
}

// GetConfiguration 获取当前配置
func (m *CacheMonitor) GetConfiguration() *MonitoringConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// 返回配置的副本
	config := *m.config
	return &config
}

// UpdateConfiguration 更新配置
func (m *CacheMonitor) UpdateConfiguration(config *MonitoringConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.config = config
	return nil
}

// GetMonitoringStats 获取监控器自身的统计
func (m *CacheMonitor) GetMonitoringStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return map[string]interface{}{
		"monitor_running":       m.isRunning,
		"http_server_enabled":   m.config.EnableHTTPEndpoints,
		"http_port":             m.config.HTTPPort,
		"alert_callbacks_count": len(m.alertCallbacks),
		"unique_alerts_seen":    len(m.lastAlerts),
		"metrics_interval":      m.config.MetricsInterval.String(),
		"health_check_interval": m.config.HealthCheckInterval.String(),
	}
}

// SimulateLoad 模拟负载（用于测试）
func (m *CacheMonitor) SimulateLoad(ctx context.Context, duration time.Duration, opsPerSecond int) {
	ticker := time.NewTicker(time.Second / time.Duration(opsPerSecond))
	defer ticker.Stop()
	
	timeout := time.After(duration)
	counter := 0
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-timeout:
			return
		case <-ticker.C:
			key := fmt.Sprintf("test_key_%d", counter%100)
			data := fmt.Sprintf("test_data_%d", counter)
			
			// 模拟随机操作
			switch counter % 4 {
			case 0, 1: // 60% get操作
				m.cacheManager.Get(key)
			case 2: // 25% set操作
				m.cacheManager.Set(key, data)
			case 3: // 15% delete操作
				m.cacheManager.Delete(key)
			}
			
			counter++
		}
	}
}