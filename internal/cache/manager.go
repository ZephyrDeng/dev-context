package cache

import (
	"context"
	"errors"
	"sync"
	"time"
)

// CacheMetrics 缓存性能指标
type CacheMetrics struct {
	mu          sync.RWMutex
	Hits        int64     `json:"hits"`
	Misses      int64     `json:"misses"`
	Sets        int64     `json:"sets"`
	Deletes     int64     `json:"deletes"`
	Evictions   int64     `json:"evictions"`
	StartTime   time.Time `json:"start_time"`
	LastReset   time.Time `json:"last_reset"`
}

// HitRate 计算命中率
func (m *CacheMetrics) HitRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	total := m.Hits + m.Misses
	if total == 0 {
		return 0.0
	}
	return float64(m.Hits) / float64(total) * 100
}

// Reset 重置指标
func (m *CacheMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.Hits = 0
	m.Misses = 0
	m.Sets = 0
	m.Deletes = 0
	m.Evictions = 0
	m.LastReset = time.Now()
}

// GetStats 获取指标统计
func (m *CacheMetrics) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return map[string]interface{}{
		"hits":      m.Hits,
		"misses":    m.Misses,
		"sets":      m.Sets,
		"deletes":   m.Deletes,
		"evictions": m.Evictions,
		"hit_rate":  m.HitRate(),
		"uptime":    time.Since(m.StartTime).Seconds(),
	}
}

// CacheManager 缓存管理器
type CacheManager struct {
	storage            *CacheStorage
	metrics            *CacheMetrics
	queryCoalescer     *QueryCoalescer
	concurrencyManager *ConcurrencyManager
	ttl                time.Duration
	cleanupInterval    time.Duration
	stopCleanup        chan struct{}
	cleanupStopped     chan struct{}
}

// CacheConfig 缓存配置
type CacheConfig struct {
	MaxSize            int64                  `json:"max_size"`            // 最大缓存大小（字节）
	TTL                time.Duration          `json:"ttl"`                 // 缓存生存时间
	CleanupInterval    time.Duration          `json:"cleanup_interval"`    // 清理间隔
	CoalescingConfig   *CoalescingConfig      `json:"coalescing_config"`   // 查询合并配置
	ConcurrencyConfig  *ConcurrencyConfig     `json:"concurrency_config"`  // 并发控制配置
}

// DefaultCacheConfig 默认缓存配置
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		MaxSize:           512 * 1024 * 1024, // 512MB
		TTL:               15 * time.Minute,   // 15分钟
		CleanupInterval:   5 * time.Minute,    // 5分钟清理一次
		CoalescingConfig:  DefaultCoalescingConfig(),
		ConcurrencyConfig: DefaultConcurrencyConfig(),
	}
}

// NewCacheManager 创建新的缓存管理器
func NewCacheManager(config *CacheConfig) *CacheManager {
	if config == nil {
		config = DefaultCacheConfig()
	}
	
	cm := &CacheManager{
		storage:            NewCacheStorage(config.MaxSize),
		metrics:            &CacheMetrics{StartTime: time.Now()},
		queryCoalescer:     NewQueryCoalescer(config.CoalescingConfig),
		concurrencyManager: NewConcurrencyManager(config.ConcurrencyConfig),
		ttl:                config.TTL,
		cleanupInterval:    config.CleanupInterval,
		stopCleanup:        make(chan struct{}),
		cleanupStopped:     make(chan struct{}),
	}
	
	// 启动后台清理协程
	go cm.backgroundCleanup()
	
	return cm
}

// Get 获取缓存数据
func (cm *CacheManager) Get(key string) (interface{}, bool) {
	start := time.Now()
	defer func() {
		// 记录响应时间
		_ = time.Since(start)
	}()
	
	result, found := cm.storage.Get(key)
	
	cm.metrics.mu.Lock()
	if found {
		cm.metrics.Hits++
	} else {
		cm.metrics.Misses++
	}
	cm.metrics.mu.Unlock()
	
	if found {
		return result.Data, true
	}
	
	return nil, false
}

// Set 设置缓存数据
func (cm *CacheManager) Set(key string, data interface{}) error {
	err := cm.storage.Set(key, data, cm.ttl)
	
	cm.metrics.mu.Lock()
	cm.metrics.Sets++
	cm.metrics.mu.Unlock()
	
	return err
}

// SetWithTTL 设置具有自定义TTL的缓存数据
func (cm *CacheManager) SetWithTTL(key string, data interface{}, ttl time.Duration) error {
	err := cm.storage.Set(key, data, ttl)
	
	cm.metrics.mu.Lock()
	cm.metrics.Sets++
	cm.metrics.mu.Unlock()
	
	return err
}

// Delete 删除缓存数据
func (cm *CacheManager) Delete(key string) bool {
	deleted := cm.storage.Delete(key)
	
	if deleted {
		cm.metrics.mu.Lock()
		cm.metrics.Deletes++
		cm.metrics.mu.Unlock()
	}
	
	return deleted
}

// Clear 清空缓存
func (cm *CacheManager) Clear() {
	cm.storage.Clear()
	cm.metrics.Reset()
}

// GetOrSet 获取缓存，如果不存在则执行函数并缓存结果
func (cm *CacheManager) GetOrSet(ctx context.Context, key string, fn func(ctx context.Context) (interface{}, error)) (interface{}, error) {
	// 先尝试获取缓存
	if data, found := cm.Get(key); found {
		return data, nil
	}
	
	// 使用新的查询合并器
	return cm.queryCoalescer.Execute(ctx, key, func(ctx context.Context) (interface{}, error) {
		// 再次检查缓存（双重检查）
		if data, found := cm.Get(key); found {
			return data, nil
		}
		
		// 执行函数获取数据
		data, err := fn(ctx)
		if err != nil {
			return nil, err
		}
		
		// 缓存结果
		if setErr := cm.Set(key, data); setErr != nil {
			// 缓存失败，但返回数据
			return data, nil
		}
		
		return data, nil
	})
}

// GetOrSetWithConcurrency 在并发控制下获取或设置缓存
func (cm *CacheManager) GetOrSetWithConcurrency(ctx context.Context, key string, fn func(ctx context.Context) (interface{}, error)) (interface{}, error) {
	// 先尝试获取缓存
	if data, found := cm.Get(key); found {
		return data, nil
	}
	
	// 在并发限制下执行
	var result interface{}
	var err error
	
	execErr := cm.concurrencyManager.ExecuteWithLimits(ctx, func() error {
		// 再次检查缓存
		if data, found := cm.Get(key); found {
			result = data
			return nil
		}
		
		// 使用查询合并器执行
		data, execErr := cm.queryCoalescer.Execute(ctx, key, func(ctx context.Context) (interface{}, error) {
			return fn(ctx)
		})
		
		if execErr != nil {
			err = execErr
			return execErr
		}
		
		// 缓存结果
		if setErr := cm.Set(key, data); setErr != nil {
			// 缓存失败，但返回数据
			result = data
			return nil
		}
		
		result = data
		return nil
	})
	
	if execErr != nil {
		return nil, execErr
	}
	
	if err != nil {
		return nil, err
	}
	
	return result, nil
}

// backgroundCleanup 后台清理过期项
func (cm *CacheManager) backgroundCleanup() {
	ticker := time.NewTicker(cm.cleanupInterval)
	defer ticker.Stop()
	defer close(cm.cleanupStopped)
	
	for {
		select {
		case <-ticker.C:
			cm.cleanupExpired()
		case <-cm.stopCleanup:
			return
		}
	}
}

// cleanupExpired 清理过期项
func (cm *CacheManager) cleanupExpired() {
	keys := cm.storage.GetKeys()
	cleaned := 0
	
	for _, key := range keys {
		if result, exists := cm.storage.Get(key); !exists {
			// 项目在Get时已被清理（过期）
			cleaned++
		} else if result.IsExpired() {
			// 手动清理过期项
			cm.storage.Delete(key)
			cleaned++
		}
	}
	
	if cleaned > 0 {
		cm.metrics.mu.Lock()
		cm.metrics.Evictions += int64(cleaned)
		cm.metrics.mu.Unlock()
	}
}

// Warmup 缓存预热
func (cm *CacheManager) Warmup(ctx context.Context, data map[string]interface{}) error {
	if data == nil {
		return errors.New("warmup data cannot be nil")
	}
	
	for key, value := range data {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := cm.Set(key, value); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// GetStats 获取缓存统计信息
func (cm *CacheManager) GetStats() map[string]interface{} {
	metricsStats := cm.metrics.GetStats()
	storageStats := cm.storage.GetStats()
	coalescingStats := cm.queryCoalescer.GetStats()
	concurrencyStats := cm.concurrencyManager.GetStats()
	
	// 合并统计信息
	stats := make(map[string]interface{})
	for k, v := range metricsStats {
		stats[k] = v
	}
	for k, v := range storageStats {
		stats[k] = v
	}
	for k, v := range coalescingStats {
		stats["coalescing_"+k] = v
	}
	for k, v := range concurrencyStats {
		stats["concurrency_"+k] = v
	}
	
	return stats
}

// Close 关闭缓存管理器
func (cm *CacheManager) Close() error {
	// 停止后台清理
	close(cm.stopCleanup)
	
	// 等待清理协程结束
	select {
	case <-cm.cleanupStopped:
	case <-time.After(5 * time.Second):
		// 超时等待
	}
	
	// 关闭查询合并器
	if err := cm.queryCoalescer.Close(); err != nil {
		return err
	}
	
	// 关闭并发管理器
	if err := cm.concurrencyManager.Close(); err != nil {
		return err
	}
	
	return nil
}

// Size 返回缓存项数量
func (cm *CacheManager) Size() int {
	return cm.storage.Size()
}

// TotalSize 返回缓存总大小
func (cm *CacheManager) TotalSize() int64 {
	return cm.storage.TotalSize()
}

// MaxSize 返回最大缓存大小
func (cm *CacheManager) MaxSize() int64 {
	return cm.storage.MaxSize()
}

// GetKeys 返回所有缓存键
func (cm *CacheManager) GetKeys() []string {
	return cm.storage.GetKeys()
}

// SetCleanupInterval 设置清理间隔
func (cm *CacheManager) SetCleanupInterval(interval time.Duration) {
	cm.cleanupInterval = interval
	// 重启清理协程
	close(cm.stopCleanup)
	<-cm.cleanupStopped
	
	cm.stopCleanup = make(chan struct{})
	cm.cleanupStopped = make(chan struct{})
	go cm.backgroundCleanup()
}

// ForceCleanup 强制执行清理
func (cm *CacheManager) ForceCleanup() {
	cm.cleanupExpired()
}

// SubmitToWorkerPool 提交任务到工作池
func (cm *CacheManager) SubmitToWorkerPool(task func()) error {
	return cm.concurrencyManager.SubmitToWorkerPool(task)
}

// GetCoalescingStats 获取查询合并统计
func (cm *CacheManager) GetCoalescingStats() map[string]interface{} {
	return cm.queryCoalescer.GetStats()
}

// GetConcurrencyStats 获取并发控制统计
func (cm *CacheManager) GetConcurrencyStats() map[string]interface{} {
	return cm.concurrencyManager.GetStats()
}

// HasActiveCoalescingGroup 检查是否有活跃的合并组
func (cm *CacheManager) HasActiveCoalescingGroup(key string) bool {
	return cm.queryCoalescer.HasActiveGroup(key)
}

// ActiveCoalescingGroupsCount 返回活跃合并组数量
func (cm *CacheManager) ActiveCoalescingGroupsCount() int {
	return cm.queryCoalescer.ActiveGroupsCount()
}