package cache

import (
	"context"
	"errors"
	"sync"
	"time"
)

// CoalescingResult 查询合并结果
type CoalescingResult struct {
	result *CachedResult
	err    error
}

// CoalescingGroup 查询合并组
type CoalescingGroup struct {
	result       chan CoalescingResult
	count        int
	timeout      time.Duration
	created      time.Time
}

// NewCoalescingGroup 创建新的查询合并组
func NewCoalescingGroup(timeout time.Duration) *CoalescingGroup {
	return &CoalescingGroup{
		result:  make(chan CoalescingResult),
		count:   1,
		timeout: timeout,
		created: time.Now(),
	}
}

// IsExpired 检查合并组是否已过期
func (g *CoalescingGroup) IsExpired() bool {
	return time.Since(g.created) > g.timeout
}

// QueryCoalescer 查询合并器
type QueryCoalescer struct {
	groups  map[string]*CoalescingGroup
	mutex   sync.Mutex
	timeout time.Duration
	stats   CoalescingStats
}

// CoalescingStats 查询合并统计
type CoalescingStats struct {
	mu             sync.RWMutex
	TotalRequests  int64 `json:"total_requests"`  // 总请求数
	MergedRequests int64 `json:"merged_requests"` // 合并的请求数
	ActiveGroups   int64 `json:"active_groups"`   // 当前活跃的合并组数
	SavedQueries   int64 `json:"saved_queries"`   // 节省的查询次数
}

// GetSavingsRate 获取节省率
func (s *CoalescingStats) GetSavingsRate() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.TotalRequests == 0 {
		return 0.0
	}
	return float64(s.SavedQueries) / float64(s.TotalRequests) * 100
}

// Reset 重置统计
func (s *CoalescingStats) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.TotalRequests = 0
	s.MergedRequests = 0
	s.ActiveGroups = 0
	s.SavedQueries = 0
}

// GetStats 获取统计信息
func (s *CoalescingStats) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return map[string]interface{}{
		"total_requests":  s.TotalRequests,
		"merged_requests": s.MergedRequests,
		"active_groups":   s.ActiveGroups,
		"saved_queries":   s.SavedQueries,
		"savings_rate":    s.GetSavingsRate(),
	}
}

// CoalescingConfig 查询合并配置
type CoalescingConfig struct {
	Timeout      time.Duration `json:"timeout"`       // 合并组超时时间
	CleanupDelay time.Duration `json:"cleanup_delay"` // 清理延迟时间
}

// DefaultCoalescingConfig 默认查询合并配置
func DefaultCoalescingConfig() *CoalescingConfig {
	return &CoalescingConfig{
		Timeout:      30 * time.Second, // 30秒超时
		CleanupDelay: 1 * time.Minute,  // 1分钟清理延迟
	}
}

// NewQueryCoalescer 创建新的查询合并器
func NewQueryCoalescer(config *CoalescingConfig) *QueryCoalescer {
	if config == nil {
		config = DefaultCoalescingConfig()
	}
	
	qc := &QueryCoalescer{
		groups:  make(map[string]*CoalescingGroup),
		timeout: config.Timeout,
	}
	
	// 启动后台清理协程
	go qc.backgroundCleanup(config.CleanupDelay)
	
	return qc
}

// Execute 执行查询合并逻辑
func (qc *QueryCoalescer) Execute(ctx context.Context, key string, fn func(ctx context.Context) (interface{}, error)) (interface{}, error) {
	qc.mutex.Lock()
	
	// 检查是否已关闭
	if qc.groups == nil {
		qc.mutex.Unlock()
		return nil, errors.New("query coalescer is closed")
	}
	
	// 更新统计
	qc.stats.mu.Lock()
	qc.stats.TotalRequests++
	qc.stats.mu.Unlock()
	
	// 检查是否已有相同查询在执行
	if group, exists := qc.groups[key]; exists && !group.IsExpired() {
		// 有相同查询在执行，等待结果
		group.count++
		qc.stats.mu.Lock()
		qc.stats.MergedRequests++
		qc.stats.SavedQueries++
		qc.stats.mu.Unlock()
		
		qc.mutex.Unlock()
		
		select {
		case result := <-group.result:
			if result.err != nil {
				return nil, result.err
			}
			if result.result != nil {
				return result.result.Data, nil
			}
			return nil, errors.New("unexpected nil result")
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	
	// 清理过期的合并组
	if group, exists := qc.groups[key]; exists && group.IsExpired() {
		delete(qc.groups, key)
	}
	
	// 创建新的查询合并组
	group := NewCoalescingGroup(qc.timeout)
	qc.groups[key] = group
	
	qc.stats.mu.Lock()
	qc.stats.ActiveGroups++
	qc.stats.mu.Unlock()
	
	qc.mutex.Unlock()
	
	// 启动后台执行goroutine
	go qc.executeQuery(ctx, key, fn, group)
	
	// 等待结果
	select {
	case result := <-group.result:
		if result.err != nil {
			return nil, result.err
		}
		if result.result != nil {
			return result.result.Data, nil
		}
		return nil, errors.New("unexpected nil result")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// executeQuery 在后台执行查询
func (qc *QueryCoalescer) executeQuery(ctx context.Context, key string, fn func(ctx context.Context) (interface{}, error), group *CoalescingGroup) {
	defer func() {
		qc.mutex.Lock()
		delete(qc.groups, key)
		qc.stats.mu.Lock()
		qc.stats.ActiveGroups--
		qc.stats.mu.Unlock()
		qc.mutex.Unlock()
	}()
	
	// 执行函数获取数据
	data, err := fn(ctx)
	if err != nil {
		result := CoalescingResult{
			result: nil,
			err:    err,
		}
		qc.broadcastResult(group, result)
		return
	}
	
	result := CoalescingResult{
		result: &CachedResult{Data: data},
		err:    nil,
	}
	qc.broadcastResult(group, result)
}

// broadcastResult 广播结果到所有等待的goroutine
func (qc *QueryCoalescer) broadcastResult(group *CoalescingGroup, result CoalescingResult) {
	// 发送结果到所有等待的goroutine
	for i := 0; i < group.count; i++ {
		select {
		case group.result <- result:
		case <-time.After(1 * time.Second):
			// 如果发送超时，跳过这个发送
		}
	}
}

// backgroundCleanup 后台清理过期的合并组
func (qc *QueryCoalescer) backgroundCleanup(cleanupDelay time.Duration) {
	ticker := time.NewTicker(cleanupDelay)
	defer ticker.Stop()
	
	for range ticker.C {
		qc.cleanupExpiredGroups()
	}
}

// cleanupExpiredGroups 清理过期的合并组
func (qc *QueryCoalescer) cleanupExpiredGroups() {
	qc.mutex.Lock()
	defer qc.mutex.Unlock()
	
	cleanedCount := 0
	for key, group := range qc.groups {
		if group.IsExpired() {
			// 向过期组发送超时错误
			timeoutResult := CoalescingResult{
				result: nil,
				err:    errors.New("coalescing group timeout"),
			}
			
			// 非阻塞地发送超时结果
			go func(g *CoalescingGroup, r CoalescingResult) {
				select {
				case g.result <- r:
				default:
				}
			}(group, timeoutResult)
			
			delete(qc.groups, key)
			cleanedCount++
		}
	}
	
	if cleanedCount > 0 {
		qc.stats.mu.Lock()
		qc.stats.ActiveGroups -= int64(cleanedCount)
		qc.stats.mu.Unlock()
	}
}

// GetStats 获取查询合并统计信息
func (qc *QueryCoalescer) GetStats() map[string]interface{} {
	stats := qc.stats.GetStats()
	
	// 添加当前活跃组数
	qc.mutex.Lock()
	activeGroups := int64(len(qc.groups))
	qc.mutex.Unlock()
	
	stats["current_active_groups"] = activeGroups
	
	return stats
}

// Close 关闭查询合并器
func (qc *QueryCoalescer) Close() error {
	qc.mutex.Lock()
	defer qc.mutex.Unlock()
	
	// 向所有活跃的合并组发送关闭信号
	for key, group := range qc.groups {
		closeResult := CoalescingResult{
			result: nil,
			err:    errors.New("query coalescer is closing"),
		}
		
		// 非阻塞地发送关闭结果
		go func(g *CoalescingGroup, r CoalescingResult) {
			for i := 0; i < g.count; i++ {
				select {
				case g.result <- r:
				default:
				}
			}
		}(group, closeResult)
		
		delete(qc.groups, key)
	}
	
	// 标记为已关闭
	qc.groups = nil
	
	return nil
}

// ActiveGroupsCount 返回当前活跃的合并组数量
func (qc *QueryCoalescer) ActiveGroupsCount() int {
	qc.mutex.Lock()
	defer qc.mutex.Unlock()
	
	return len(qc.groups)
}

// HasActiveGroup 检查是否有特定key的活跃合并组
func (qc *QueryCoalescer) HasActiveGroup(key string) bool {
	qc.mutex.Lock()
	defer qc.mutex.Unlock()
	
	group, exists := qc.groups[key]
	return exists && !group.IsExpired()
}