package cache

import (
	"context"
	"sync"
	"time"
)

// CleanupStrategy 清理策略
type CleanupStrategy int

const (
	// StrategyTTLOnly 只清理过期项
	StrategyTTLOnly CleanupStrategy = iota
	// StrategyLRU LRU清理策略
	StrategyLRU
	// StrategySize 基于大小的清理策略
	StrategySize
	// StrategyMixed 混合策略（TTL + LRU + Size）
	StrategyMixed
)

// CleanupItem 清理项信息
type CleanupItem struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	AccessCount  int64     `json:"access_count"`
	LastAccessed time.Time `json:"last_accessed"`
	CreatedAt    time.Time `json:"created_at"`
	Expiry       time.Time `json:"expiry"`
	Priority     int       `json:"priority"` // 清理优先级，越高越优先清理
}

// ShouldClean 判断是否应该清理这个项目
func (item *CleanupItem) ShouldClean(strategy CleanupStrategy, threshold map[string]interface{}) bool {
	now := time.Now()
	
	switch strategy {
	case StrategyTTLOnly:
		return now.After(item.Expiry)
		
	case StrategyLRU:
		if maxAge, ok := threshold["max_age"].(time.Duration); ok {
			return now.Sub(item.LastAccessed) > maxAge
		}
		return false
		
	case StrategySize:
		if minSize, ok := threshold["min_size"].(int64); ok {
			return item.Size >= minSize
		}
		return false
		
	case StrategyMixed:
		// 过期项优先清理
		if now.After(item.Expiry) {
			return true
		}
		// 检查LRU条件
		if maxAge, ok := threshold["max_age"].(time.Duration); ok {
			if now.Sub(item.LastAccessed) > maxAge {
				return true
			}
		}
		// 检查大小条件
		if minSize, ok := threshold["min_size"].(int64); ok {
			if item.Size >= minSize {
				return true
			}
		}
		return false
	}
	
	return false
}

// CalculatePriority 计算清理优先级
func (item *CleanupItem) CalculatePriority() int {
	now := time.Now()
	priority := 0
	
	// 过期项优先级最高
	if now.After(item.Expiry) {
		priority += 1000
	}
	
	// 根据最后访问时间增加优先级
	daysSinceAccess := now.Sub(item.LastAccessed).Hours() / 24
	if daysSinceAccess > 0 {
		priority += int(daysSinceAccess * 10)
	}
	
	// 根据访问频率降低优先级（但不能低于0）
	if item.AccessCount > 0 && priority > int(item.AccessCount) {
		priority -= int(item.AccessCount)
	}
	
	// 根据大小增加优先级
	sizeKB := item.Size / 1024
	priority += int(sizeKB / 100) // 每100KB增加1点优先级
	
	// 确保优先级不为负
	if priority < 0 {
		priority = 0
	}
	
	item.Priority = priority
	return priority
}

// CleanupManager 清理管理器
type CleanupManager struct {
	mu                sync.RWMutex
	storage           *CacheStorage
	ttlManager        *TTLManager
	strategy          CleanupStrategy
	cleanupInterval   time.Duration
	threshold         map[string]interface{}
	maxCleanupPercent float64  // 每次清理的最大百分比
	cleanupCallback   func([]string, CleanupStrategy) // 清理回调
	stopCleanup       chan struct{}
	cleanupStopped    chan struct{}
	contextCancel     context.CancelFunc
	
	// 统计信息
	stats *CleanupStats
}

// CleanupStats 清理统计信息
type CleanupStats struct {
	mu                  sync.RWMutex
	TotalCleanups       int64     `json:"total_cleanups"`
	TotalItemsCleaned   int64     `json:"total_items_cleaned"`
	TotalSpaceFreed     int64     `json:"total_space_freed"`
	LastCleanupTime     time.Time `json:"last_cleanup_time"`
	LastCleanupDuration time.Duration `json:"last_cleanup_duration"`
	AverageCleanupTime  time.Duration `json:"average_cleanup_time"`
	StrategyUsage       map[CleanupStrategy]int64 `json:"strategy_usage"`
}

// NewCleanupStats 创建新的清理统计
func NewCleanupStats() *CleanupStats {
	return &CleanupStats{
		StrategyUsage: make(map[CleanupStrategy]int64),
	}
}

// RecordCleanup 记录清理操作
func (s *CleanupStats) RecordCleanup(strategy CleanupStrategy, itemsCleaned int64, spaceFreed int64, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.TotalCleanups++
	s.TotalItemsCleaned += itemsCleaned
	s.TotalSpaceFreed += spaceFreed
	s.LastCleanupTime = time.Now()
	s.LastCleanupDuration = duration
	s.StrategyUsage[strategy]++
	
	// 计算平均清理时间
	if s.TotalCleanups > 1 {
		s.AverageCleanupTime = time.Duration((int64(s.AverageCleanupTime) + int64(duration)) / 2)
	} else {
		s.AverageCleanupTime = duration
	}
}

// GetStats 获取统计信息
func (s *CleanupStats) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	strategyNames := map[CleanupStrategy]string{
		StrategyTTLOnly: "ttl_only",
		StrategyLRU:     "lru",
		StrategySize:    "size",
		StrategyMixed:   "mixed",
	}
	
	strategyStats := make(map[string]int64)
	for strategy, count := range s.StrategyUsage {
		strategyStats[strategyNames[strategy]] = count
	}
	
	return map[string]interface{}{
		"total_cleanups":        s.TotalCleanups,
		"total_items_cleaned":   s.TotalItemsCleaned,
		"total_space_freed":     s.TotalSpaceFreed,
		"last_cleanup_time":     s.LastCleanupTime,
		"last_cleanup_duration": s.LastCleanupDuration.String(),
		"average_cleanup_time":  s.AverageCleanupTime.String(),
		"strategy_usage":        strategyStats,
	}
}

// CleanupConfig 清理管理器配置
type CleanupConfig struct {
	Strategy          CleanupStrategy              `json:"strategy"`
	CleanupInterval   time.Duration                `json:"cleanup_interval"`
	Threshold         map[string]interface{}       `json:"threshold"`
	MaxCleanupPercent float64                      `json:"max_cleanup_percent"`
	CleanupCallback   func([]string, CleanupStrategy) `json:"-"`
}

// DefaultCleanupConfig 返回默认清理配置
func DefaultCleanupConfig() *CleanupConfig {
	return &CleanupConfig{
		Strategy:        StrategyMixed,
		CleanupInterval: 5 * time.Minute,
		Threshold: map[string]interface{}{
			"max_age":  30 * time.Minute, // LRU最大空闲时间
			"min_size": int64(1024),      // 大小清理阈值（1KB）
		},
		MaxCleanupPercent: 0.25, // 每次最多清理25%
	}
}

// NewCleanupManager 创建新的清理管理器
func NewCleanupManager(storage *CacheStorage, ttlManager *TTLManager, config *CleanupConfig) *CleanupManager {
	if config == nil {
		config = DefaultCleanupConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	cm := &CleanupManager{
		storage:           storage,
		ttlManager:        ttlManager,
		strategy:          config.Strategy,
		cleanupInterval:   config.CleanupInterval,
		threshold:         config.Threshold,
		maxCleanupPercent: config.MaxCleanupPercent,
		cleanupCallback:   config.CleanupCallback,
		stopCleanup:       make(chan struct{}),
		cleanupStopped:    make(chan struct{}),
		contextCancel:     cancel,
		stats:             NewCleanupStats(),
	}
	
	// 启动后台清理协程
	go cm.backgroundCleanup(ctx)
	
	return cm
}

// ForceCleanup 强制执行清理
func (cm *CleanupManager) ForceCleanup() ([]string, error) {
	return cm.cleanup()
}

// cleanup 执行清理操作
func (cm *CleanupManager) cleanup() ([]string, error) {
	start := time.Now()
	
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	// 收集所有缓存项信息
	items := cm.collectCleanupItems()
	if len(items) == 0 {
		return []string{}, nil
	}
	
	// 根据策略选择要清理的项目
	itemsToClean := cm.selectItemsToClean(items)
	if len(itemsToClean) == 0 {
		return []string{}, nil
	}
	
	// 执行清理
	cleanedKeys := make([]string, 0, len(itemsToClean))
	totalSpaceFreed := int64(0)
	
	for _, item := range itemsToClean {
		// 从存储中删除
		if cm.storage.Delete(item.Key) {
			cleanedKeys = append(cleanedKeys, item.Key)
			totalSpaceFreed += item.Size
			
			// 从TTL管理器中删除
			if cm.ttlManager != nil {
				cm.ttlManager.Delete(item.Key)
			}
		}
	}
	
	// 记录统计信息
	duration := time.Since(start)
	cm.stats.RecordCleanup(cm.strategy, int64(len(cleanedKeys)), totalSpaceFreed, duration)
	
	// 调用清理回调
	if cm.cleanupCallback != nil && len(cleanedKeys) > 0 {
		go cm.cleanupCallback(cleanedKeys, cm.strategy)
	}
	
	return cleanedKeys, nil
}

// collectCleanupItems 收集清理项信息
func (cm *CleanupManager) collectCleanupItems() []*CleanupItem {
	var items []*CleanupItem
	
	// 直接访问存储数据，避免自动过期清理
	cm.storage.mutex.RLock()
	defer cm.storage.mutex.RUnlock()
	
	for key, result := range cm.storage.data {
		// 获取TTL信息
		var expiry time.Time
		if cm.ttlManager != nil {
			if entry, ttlExists := cm.ttlManager.Get(key); ttlExists {
				expiry = entry.Expiry
			} else {
				// 如果TTL管理器中没有条目，使用存储中的过期时间
				expiry = result.Expiry
			}
		} else {
			expiry = result.Expiry
		}
		
		item := &CleanupItem{
			Key:          key,
			Size:         result.EstimateSize(),
			AccessCount:  result.AccessCount,
			LastAccessed: result.Timestamp,
			CreatedAt:    result.Timestamp, // 简化处理，使用时间戳作为创建时间
			Expiry:       expiry,
		}
		
		// 计算清理优先级
		item.CalculatePriority()
		
		items = append(items, item)
	}
	
	return items
}

// selectItemsToClean 根据策略选择要清理的项目
func (cm *CleanupManager) selectItemsToClean(items []*CleanupItem) []*CleanupItem {
	var candidates []*CleanupItem
	
	// 根据策略筛选候选项
	for _, item := range items {
		if item.ShouldClean(cm.strategy, cm.threshold) {
			candidates = append(candidates, item)
		}
	}
	
	if len(candidates) == 0 {
		return []*CleanupItem{}
	}
	
	// 按优先级排序（优先级高的排在前面）
	cm.sortByPriority(candidates)
	
	// 限制清理数量（不超过总数的指定百分比）
	maxItems := int(float64(len(items)) * cm.maxCleanupPercent)
	if maxItems < 1 {
		maxItems = 1
	}
	
	if len(candidates) > maxItems {
		candidates = candidates[:maxItems]
	}
	
	return candidates
}

// sortByPriority 按优先级排序（简单的冒泡排序）
func (cm *CleanupManager) sortByPriority(items []*CleanupItem) {
	n := len(items)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if items[j].Priority < items[j+1].Priority {
				items[j], items[j+1] = items[j+1], items[j]
			}
		}
	}
}

// backgroundCleanup 后台清理协程
func (cm *CleanupManager) backgroundCleanup(ctx context.Context) {
	defer close(cm.cleanupStopped)
	
	ticker := time.NewTicker(cm.cleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-cm.stopCleanup:
			return
		case <-ticker.C:
			cm.cleanup()
		}
	}
}

// SetStrategy 设置清理策略
func (cm *CleanupManager) SetStrategy(strategy CleanupStrategy) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.strategy = strategy
}

// GetStrategy 获取当前清理策略
func (cm *CleanupManager) GetStrategy() CleanupStrategy {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.strategy
}

// SetCleanupInterval 设置清理间隔
func (cm *CleanupManager) SetCleanupInterval(interval time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.cleanupInterval = interval
	
	// 重启后台清理（发送停止信号，协程会重新创建）
	select {
	case cm.stopCleanup <- struct{}{}:
	default:
	}
}

// GetCleanupInterval 获取清理间隔
func (cm *CleanupManager) GetCleanupInterval() time.Duration {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.cleanupInterval
}

// SetThreshold 设置清理阈值
func (cm *CleanupManager) SetThreshold(threshold map[string]interface{}) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.threshold = threshold
}

// GetThreshold 获取清理阈值
func (cm *CleanupManager) GetThreshold() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	// 返回副本
	threshold := make(map[string]interface{})
	for k, v := range cm.threshold {
		threshold[k] = v
	}
	
	return threshold
}

// SetMaxCleanupPercent 设置最大清理百分比
func (cm *CleanupManager) SetMaxCleanupPercent(percent float64) {
	if percent < 0 {
		percent = 0
	}
	if percent > 1.0 {
		percent = 1.0
	}
	
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.maxCleanupPercent = percent
}

// GetMaxCleanupPercent 获取最大清理百分比
func (cm *CleanupManager) GetMaxCleanupPercent() float64 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.maxCleanupPercent
}

// GetStats 获取清理统计信息
func (cm *CleanupManager) GetStats() map[string]interface{} {
	statsData := cm.stats.GetStats()
	
	// 添加清理管理器的配置信息
	strategyName := "unknown"
	switch cm.strategy {
	case StrategyTTLOnly:
		strategyName = "ttl_only"
	case StrategyLRU:
		strategyName = "lru"
	case StrategySize:
		strategyName = "size"
	case StrategyMixed:
		strategyName = "mixed"
	}
	
	statsData["current_strategy"] = strategyName
	statsData["cleanup_interval"] = cm.cleanupInterval.String()
	statsData["max_cleanup_percent"] = cm.maxCleanupPercent
	statsData["threshold"] = cm.GetThreshold()
	
	return statsData
}

// PredictNextCleanup 预测下次清理的项目数量
func (cm *CleanupManager) PredictNextCleanup() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	items := cm.collectCleanupItems()
	candidates := cm.selectItemsToClean(items)
	
	totalSize := int64(0)
	for _, item := range candidates {
		totalSize += item.Size
	}
	
	return map[string]interface{}{
		"total_items":          len(items),
		"items_to_clean":       len(candidates),
		"space_to_free":        totalSize,
		"cleanup_percentage":   float64(len(candidates)) / float64(len(items)) * 100,
		"next_cleanup_time":    time.Now().Add(cm.cleanupInterval),
	}
}

// Close 关闭清理管理器
func (cm *CleanupManager) Close() error {
	// 停止后台清理
	close(cm.stopCleanup)
	if cm.contextCancel != nil {
		cm.contextCancel()
	}
	
	// 等待清理协程结束
	select {
	case <-cm.cleanupStopped:
	case <-time.After(5 * time.Second):
		// 超时等待
	}
	
	return nil
}

// IsRunning 检查清理管理器是否正在运行
func (cm *CleanupManager) IsRunning() bool {
	select {
	case <-cm.cleanupStopped:
		return false
	default:
		return true
	}
}