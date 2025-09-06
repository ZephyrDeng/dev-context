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

// coalescingResult 查询合并结果
type coalescingResult struct {
	result *CachedResult
	err    error
}

// coalescingGroup 查询合并组
type coalescingGroup struct {
	result chan coalescingResult
	count  int
}

// CacheManager 缓存管理器
type CacheManager struct {
	storage          *CacheStorage
	metrics          *CacheMetrics
	ttl              time.Duration
	cleanupInterval  time.Duration
	coalescing       map[string]*coalescingGroup
	coalescingMutex  sync.Mutex
	stopCleanup      chan struct{}
	cleanupStopped   chan struct{}
}

// CacheConfig 缓存配置
type CacheConfig struct {
	MaxSize         int64         `json:"max_size"`         // 最大缓存大小（字节）
	TTL             time.Duration `json:"ttl"`              // 缓存生存时间
	CleanupInterval time.Duration `json:"cleanup_interval"` // 清理间隔
}

// DefaultCacheConfig 默认缓存配置
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		MaxSize:         512 * 1024 * 1024, // 512MB
		TTL:             15 * time.Minute,   // 15分钟
		CleanupInterval: 5 * time.Minute,    // 5分钟清理一次
	}
}

// NewCacheManager 创建新的缓存管理器
func NewCacheManager(config *CacheConfig) *CacheManager {
	if config == nil {
		config = DefaultCacheConfig()
	}
	
	cm := &CacheManager{
		storage:          NewCacheStorage(config.MaxSize),
		metrics:          &CacheMetrics{StartTime: time.Now()},
		ttl:              config.TTL,
		cleanupInterval:  config.CleanupInterval,
		coalescing:       make(map[string]*coalescingGroup),
		stopCleanup:      make(chan struct{}),
		cleanupStopped:   make(chan struct{}),
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
	
	// 使用查询合并防止重复请求
	return cm.getOrSetWithCoalescing(ctx, key, fn)
}

// getOrSetWithCoalescing 带查询合并的获取或设置
func (cm *CacheManager) getOrSetWithCoalescing(ctx context.Context, key string, fn func(ctx context.Context) (interface{}, error)) (interface{}, error) {
	cm.coalescingMutex.Lock()
	
	// 检查是否已有相同查询在执行
	if group, exists := cm.coalescing[key]; exists {
		// 有相同查询在执行，等待结果
		group.count++
		cm.coalescingMutex.Unlock()
		
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
	
	// 创建新的查询合并组
	group := &coalescingGroup{
		result: make(chan coalescingResult),
		count:  1,
	}
	cm.coalescing[key] = group
	cm.coalescingMutex.Unlock()
	
	// 执行查询
	go func() {
		defer func() {
			cm.coalescingMutex.Lock()
			delete(cm.coalescing, key)
			cm.coalescingMutex.Unlock()
		}()
		
		// 再次检查缓存（可能在等待锁期间被其他协程设置）
		if data, found := cm.Get(key); found {
			result := coalescingResult{
				result: &CachedResult{Data: data},
				err:    nil,
			}
			cm.broadcastResult(group, result)
			return
		}
		
		// 执行函数获取数据
		data, err := fn(ctx)
		if err != nil {
			result := coalescingResult{
				result: nil,
				err:    err,
			}
			cm.broadcastResult(group, result)
			return
		}
		
		// 缓存结果
		if setErr := cm.Set(key, data); setErr != nil {
			// 缓存失败，但返回数据
			result := coalescingResult{
				result: &CachedResult{Data: data},
				err:    nil,
			}
			cm.broadcastResult(group, result)
			return
		}
		
		result := coalescingResult{
			result: &CachedResult{Data: data},
			err:    nil,
		}
		cm.broadcastResult(group, result)
	}()
	
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

// broadcastResult 广播结果到所有等待的goroutine
func (cm *CacheManager) broadcastResult(group *coalescingGroup, result coalescingResult) {
	// 发送结果到所有等待的goroutine
	for i := 0; i < group.count; i++ {
		select {
		case group.result <- result:
		default:
			// 如果通道已满，跳过这个发送
		}
	}
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
	
	// 合并统计信息
	stats := make(map[string]interface{})
	for k, v := range metricsStats {
		stats[k] = v
	}
	for k, v := range storageStats {
		stats[k] = v
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
	
	// 清空所有合并中的查询
	cm.coalescingMutex.Lock()
	for key, group := range cm.coalescing {
		// 发送关闭错误到所有等待的goroutine
		for i := 0; i < group.count; i++ {
			select {
			case group.result <- coalescingResult{err: errors.New("cache manager is closing")}:
			default:
			}
		}
		delete(cm.coalescing, key)
	}
	cm.coalescingMutex.Unlock()
	
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