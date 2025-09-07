package cache

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ResponseTimeStats 响应时间统计
type ResponseTimeStats struct {
	mu          sync.RWMutex
	samples     []time.Duration
	maxSamples  int
	totalTime   time.Duration
	count       int64
	minTime     time.Duration
	maxTime     time.Duration
}

// NewResponseTimeStats 创建响应时间统计实例
func NewResponseTimeStats(maxSamples int) *ResponseTimeStats {
	if maxSamples <= 0 {
		maxSamples = 1000 // 默认保留最近1000个样本
	}
	
	return &ResponseTimeStats{
		samples:    make([]time.Duration, 0, maxSamples),
		maxSamples: maxSamples,
		minTime:    time.Duration(0),
		maxTime:    time.Duration(0),
	}
}

// Record 记录一次响应时间
func (s *ResponseTimeStats) Record(duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.count++
	s.totalTime += duration
	
	// 更新最小最大值
	if s.minTime == 0 || duration < s.minTime {
		s.minTime = duration
	}
	if duration > s.maxTime {
		s.maxTime = duration
	}
	
	// 添加样本
	if len(s.samples) >= s.maxSamples {
		// 移除最老的样本
		copy(s.samples, s.samples[1:])
		s.samples[len(s.samples)-1] = duration
	} else {
		s.samples = append(s.samples, duration)
	}
}

// GetStats 获取响应时间统计信息
func (s *ResponseTimeStats) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.count == 0 {
		return map[string]interface{}{
			"count":       int64(0),
			"average_ms":  0.0,
			"min_ms":      0.0,
			"max_ms":      0.0,
			"p50_ms":      0.0,
			"p95_ms":      0.0,
			"p99_ms":      0.0,
		}
	}
	
	avgDuration := time.Duration(int64(s.totalTime) / s.count)
	
	stats := map[string]interface{}{
		"count":      s.count,
		"average_ms": float64(avgDuration.Nanoseconds()) / 1e6,
		"min_ms":     float64(s.minTime.Nanoseconds()) / 1e6,
		"max_ms":     float64(s.maxTime.Nanoseconds()) / 1e6,
	}
	
	// 计算百分位数
	if len(s.samples) > 0 {
		// 复制并排序样本
		sortedSamples := make([]time.Duration, len(s.samples))
		copy(sortedSamples, s.samples)
		sort.Slice(sortedSamples, func(i, j int) bool {
			return sortedSamples[i] < sortedSamples[j]
		})
		
		// 计算百分位数
		p50Index := len(sortedSamples) * 50 / 100
		p95Index := len(sortedSamples) * 95 / 100
		p99Index := len(sortedSamples) * 99 / 100
		
		if p50Index < len(sortedSamples) {
			stats["p50_ms"] = float64(sortedSamples[p50Index].Nanoseconds()) / 1e6
		}
		if p95Index < len(sortedSamples) {
			stats["p95_ms"] = float64(sortedSamples[p95Index].Nanoseconds()) / 1e6
		}
		if p99Index < len(sortedSamples) {
			stats["p99_ms"] = float64(sortedSamples[p99Index].Nanoseconds()) / 1e6
		}
	}
	
	return stats
}

// Clear 清除所有统计数据
func (s *ResponseTimeStats) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.samples = s.samples[:0]
	s.totalTime = 0
	s.count = 0
	s.minTime = 0
	s.maxTime = 0
}

// MemoryUsageStats 内存使用统计
type MemoryUsageStats struct {
	mu               sync.RWMutex
	currentSize      int64
	maxSize          int64
	peakSize         int64
	totalAllocations int64
	totalDeallocations int64
	sizeHistory      []int64
	maxHistory       int
}

// NewMemoryUsageStats 创建内存使用统计实例
func NewMemoryUsageStats(maxSize int64, maxHistory int) *MemoryUsageStats {
	if maxHistory <= 0 {
		maxHistory = 100 // 默认保留100个历史记录
	}
	
	return &MemoryUsageStats{
		maxSize:     maxSize,
		sizeHistory: make([]int64, 0, maxHistory),
		maxHistory:  maxHistory,
	}
}

// RecordAllocation 记录内存分配
func (s *MemoryUsageStats) RecordAllocation(size int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.currentSize += size
	s.totalAllocations++
	
	// 更新峰值
	if s.currentSize > s.peakSize {
		s.peakSize = s.currentSize
	}
	
	// 添加历史记录
	if len(s.sizeHistory) >= s.maxHistory {
		copy(s.sizeHistory, s.sizeHistory[1:])
		s.sizeHistory[len(s.sizeHistory)-1] = s.currentSize
	} else {
		s.sizeHistory = append(s.sizeHistory, s.currentSize)
	}
}

// RecordDeallocation 记录内存释放
func (s *MemoryUsageStats) RecordDeallocation(size int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.currentSize -= size
	s.totalDeallocations++
	
	// 防止负值
	if s.currentSize < 0 {
		s.currentSize = 0
	}
	
	// 添加历史记录
	if len(s.sizeHistory) >= s.maxHistory {
		copy(s.sizeHistory, s.sizeHistory[1:])
		s.sizeHistory[len(s.sizeHistory)-1] = s.currentSize
	} else {
		s.sizeHistory = append(s.sizeHistory, s.currentSize)
	}
}

// GetStats 获取内存使用统计
func (s *MemoryUsageStats) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	usagePercent := float64(0)
	if s.maxSize > 0 {
		usagePercent = float64(s.currentSize) / float64(s.maxSize) * 100
	}
	
	peakPercent := float64(0)
	if s.maxSize > 0 {
		peakPercent = float64(s.peakSize) / float64(s.maxSize) * 100
	}
	
	return map[string]interface{}{
		"current_size_bytes":    s.currentSize,
		"max_size_bytes":        s.maxSize,
		"peak_size_bytes":       s.peakSize,
		"usage_percent":         usagePercent,
		"peak_usage_percent":    peakPercent,
		"total_allocations":     s.totalAllocations,
		"total_deallocations":   s.totalDeallocations,
		"net_allocations":       s.totalAllocations - s.totalDeallocations,
	}
}

// DetailedCacheMetrics 扩展的缓存指标
type DetailedCacheMetrics struct {
	*CacheMetrics                    // 嵌入基础指标
	
	// 详细的性能统计
	GetResponseTime    *ResponseTimeStats
	SetResponseTime    *ResponseTimeStats
	DeleteResponseTime *ResponseTimeStats
	MemoryUsage        *MemoryUsageStats
	
	// 高级统计
	mu                   sync.RWMutex
	KeyAccessFrequency   map[string]int64  `json:"key_access_frequency"`
	HourlyStats          map[int]int64     `json:"hourly_stats"`       // 按小时统计命中数
	TypeStats            map[string]int64  `json:"type_stats"`         // 按数据类型统计
	SizeDistribution     map[string]int64  `json:"size_distribution"`  // 大小分布统计
	ErrorCount           int64             `json:"error_count"`
	LastErrorTime        time.Time         `json:"last_error_time"`
	LastError            string            `json:"last_error"`
	
	// 性能阈值监控
	SlowOperationThreshold time.Duration
	SlowOperationCount     int64
	LargeItemThreshold     int64
	LargeItemCount         int64
}

// NewDetailedCacheMetrics 创建详细的缓存指标实例
func NewDetailedCacheMetrics(maxSize int64) *DetailedCacheMetrics {
	baseMetrics := &CacheMetrics{
		StartTime: time.Now(),
		LastReset: time.Now(),
	}
	
	return &DetailedCacheMetrics{
		CacheMetrics:           baseMetrics,
		GetResponseTime:        NewResponseTimeStats(1000),
		SetResponseTime:        NewResponseTimeStats(1000),
		DeleteResponseTime:     NewResponseTimeStats(1000),
		MemoryUsage:            NewMemoryUsageStats(maxSize, 100),
		KeyAccessFrequency:     make(map[string]int64),
		HourlyStats:            make(map[int]int64),
		TypeStats:              make(map[string]int64),
		SizeDistribution:       make(map[string]int64),
		SlowOperationThreshold: 100 * time.Millisecond, // 默认100ms为慢操作
		LargeItemThreshold:     1024 * 1024,            // 默认1MB为大项
	}
}

// RecordGet 记录Get操作
func (m *DetailedCacheMetrics) RecordGet(key string, hit bool, duration time.Duration, size int64) {
	m.CacheMetrics.mu.Lock()
	if hit {
		m.CacheMetrics.Hits++
	} else {
		m.CacheMetrics.Misses++
	}
	m.CacheMetrics.mu.Unlock()
	
	m.GetResponseTime.Record(duration)
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 记录键访问频率
	m.KeyAccessFrequency[key]++
	
	// 记录小时统计
	hour := time.Now().Hour()
	if hit {
		m.HourlyStats[hour]++
	}
	
	// 慢操作检查
	if duration > m.SlowOperationThreshold {
		m.SlowOperationCount++
	}
	
	// 大项检查
	if size > m.LargeItemThreshold {
		m.LargeItemCount++
	}
	
	// 大小分布
	m.recordSizeDistribution(size)
}

// RecordSet 记录Set操作
func (m *DetailedCacheMetrics) RecordSet(key string, duration time.Duration, size int64, dataType string) {
	m.CacheMetrics.mu.Lock()
	m.CacheMetrics.Sets++
	m.CacheMetrics.mu.Unlock()
	
	m.SetResponseTime.Record(duration)
	m.MemoryUsage.RecordAllocation(size)
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 记录数据类型统计
	m.TypeStats[dataType]++
	
	// 慢操作检查
	if duration > m.SlowOperationThreshold {
		m.SlowOperationCount++
	}
	
	// 大项检查
	if size > m.LargeItemThreshold {
		m.LargeItemCount++
	}
	
	// 大小分布
	m.recordSizeDistribution(size)
}

// RecordDelete 记录Delete操作
func (m *DetailedCacheMetrics) RecordDelete(key string, duration time.Duration, size int64) {
	m.CacheMetrics.mu.Lock()
	m.CacheMetrics.Deletes++
	m.CacheMetrics.mu.Unlock()
	
	m.DeleteResponseTime.Record(duration)
	if size > 0 {
		m.MemoryUsage.RecordDeallocation(size)
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 从访问频率中删除
	delete(m.KeyAccessFrequency, key)
	
	// 慢操作检查
	if duration > m.SlowOperationThreshold {
		m.SlowOperationCount++
	}
}

// RecordEviction 记录淘汰操作
func (m *DetailedCacheMetrics) RecordEviction(count int, totalSize int64) {
	m.CacheMetrics.mu.Lock()
	m.CacheMetrics.Evictions += int64(count)
	m.CacheMetrics.mu.Unlock()
	
	m.MemoryUsage.RecordDeallocation(totalSize)
}

// RecordError 记录错误
func (m *DetailedCacheMetrics) RecordError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.ErrorCount++
	m.LastErrorTime = time.Now()
	m.LastError = err.Error()
}

// recordSizeDistribution 记录大小分布（内部方法，调用时需要持有锁）
func (m *DetailedCacheMetrics) recordSizeDistribution(size int64) {
	switch {
	case size <= 1024:
		m.SizeDistribution["<=1KB"]++
	case size <= 10*1024:
		m.SizeDistribution["<=10KB"]++
	case size <= 100*1024:
		m.SizeDistribution["<=100KB"]++
	case size <= 1024*1024:
		m.SizeDistribution["<=1MB"]++
	case size <= 10*1024*1024:
		m.SizeDistribution["<=10MB"]++
	default:
		m.SizeDistribution[">10MB"]++
	}
}

// GetDetailedStats 获取详细统计信息
func (m *DetailedCacheMetrics) GetDetailedStats() map[string]interface{} {
	// 获取基础统计
	baseStats := m.CacheMetrics.GetStats()
	
	// 获取响应时间统计
	getStats := m.GetResponseTime.GetStats()
	setStats := m.SetResponseTime.GetStats()
	deleteStats := m.DeleteResponseTime.GetStats()
	
	// 获取内存统计
	memoryStats := m.MemoryUsage.GetStats()
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// 合并所有统计
	detailedStats := make(map[string]interface{})
	
	// 基础统计
	for k, v := range baseStats {
		detailedStats[k] = v
	}
	
	// 响应时间统计
	detailedStats["get_response_time"] = getStats
	detailedStats["set_response_time"] = setStats
	detailedStats["delete_response_time"] = deleteStats
	
	// 内存统计
	for k, v := range memoryStats {
		detailedStats["memory_"+k] = v
	}
	
	// 访问频率前10
	topKeys := m.getTopKeys(10)
	detailedStats["top_accessed_keys"] = topKeys
	
	// 高级统计
	detailedStats["hourly_hits"] = m.copyIntMapFromHourly(m.HourlyStats)
	detailedStats["type_distribution"] = m.copyIntMap(m.TypeStats)
	detailedStats["size_distribution"] = m.copyIntMap(m.SizeDistribution)
	detailedStats["error_count"] = m.ErrorCount
	detailedStats["last_error"] = m.LastError
	detailedStats["last_error_time"] = m.LastErrorTime
	detailedStats["slow_operation_count"] = m.SlowOperationCount
	detailedStats["large_item_count"] = m.LargeItemCount
	detailedStats["slow_operation_threshold_ms"] = float64(m.SlowOperationThreshold.Nanoseconds()) / 1e6
	detailedStats["large_item_threshold_bytes"] = m.LargeItemThreshold
	
	return detailedStats
}

// getTopKeys 获取访问频率最高的键（内部方法，调用时需要持有锁）
func (m *DetailedCacheMetrics) getTopKeys(limit int) []map[string]interface{} {
	type keyFreq struct {
		key   string
		count int64
	}
	
	var pairs []keyFreq
	for key, count := range m.KeyAccessFrequency {
		pairs = append(pairs, keyFreq{key, count})
	}
	
	// 按频率排序
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].count > pairs[j].count
	})
	
	// 取前N个
	if limit > len(pairs) {
		limit = len(pairs)
	}
	
	result := make([]map[string]interface{}, limit)
	for i := 0; i < limit; i++ {
		result[i] = map[string]interface{}{
			"key":   pairs[i].key,
			"count": pairs[i].count,
		}
	}
	
	return result
}

// copyIntMap 复制整数映射（内部方法）
func (m *DetailedCacheMetrics) copyIntMap(src map[string]int64) map[string]int64 {
	dst := make(map[string]int64)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// copyIntMapFromHourly 从小时映射复制（内部方法）
func (m *DetailedCacheMetrics) copyIntMapFromHourly(src map[int]int64) map[string]int64 {
	dst := make(map[string]int64)
	for k, v := range src {
		dst[fmt.Sprintf("%d", k)] = v
	}
	return dst
}

// Reset 重置所有指标
func (m *DetailedCacheMetrics) Reset() {
	m.CacheMetrics.Reset()
	
	m.GetResponseTime.Clear()
	m.SetResponseTime.Clear()
	m.DeleteResponseTime.Clear()
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.KeyAccessFrequency = make(map[string]int64)
	m.HourlyStats = make(map[int]int64)
	m.TypeStats = make(map[string]int64)
	m.SizeDistribution = make(map[string]int64)
	m.ErrorCount = 0
	m.LastError = ""
	m.SlowOperationCount = 0
	m.LargeItemCount = 0
}

// SetThresholds 设置性能阈值
func (m *DetailedCacheMetrics) SetThresholds(slowOpThreshold time.Duration, largeItemThreshold int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.SlowOperationThreshold = slowOpThreshold
	m.LargeItemThreshold = largeItemThreshold
}

// GetHealthStatus 获取缓存健康状态
func (m *DetailedCacheMetrics) GetHealthStatus() map[string]interface{} {
	stats := m.GetDetailedStats()
	
	status := "healthy"
	warnings := []string{}
	errors := []string{}
	
	// 检查命中率
	if hitRate, ok := stats["hit_rate"].(float64); ok && hitRate < 50 {
		warnings = append(warnings, "低命中率: "+fmt.Sprintf("%.2f%%", hitRate))
		if hitRate < 20 {
			status = "degraded"
		}
	}
	
	// 检查内存使用率
	if usage, ok := stats["memory_usage_percent"].(float64); ok && usage > 80 {
		warnings = append(warnings, "高内存使用率: "+fmt.Sprintf("%.2f%%", usage))
		if usage > 95 {
			status = "critical"
			errors = append(errors, "内存使用率过高")
		}
	}
	
	// 检查错误率
	if errorCount, ok := stats["error_count"].(int64); ok && errorCount > 0 {
		warnings = append(warnings, fmt.Sprintf("发现 %d 个错误", errorCount))
	}
	
	// 检查慢操作
	if slowOps, ok := stats["slow_operation_count"].(int64); ok && slowOps > 0 {
		warnings = append(warnings, fmt.Sprintf("发现 %d 个慢操作", slowOps))
	}
	
	return map[string]interface{}{
		"status":    status,
		"warnings":  warnings,
		"errors":    errors,
		"timestamp": time.Now(),
		"uptime":    time.Since(m.CacheMetrics.StartTime).Seconds(),
	}
}

// StartPeriodicReporting 启动定期报告
func (m *DetailedCacheMetrics) StartPeriodicReporting(ctx context.Context, interval time.Duration, callback func(stats map[string]interface{})) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats := m.GetDetailedStats()
			callback(stats)
		}
	}
}