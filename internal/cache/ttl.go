package cache

import (
	"context"
	"sync"
	"time"
)

// TTLEntry TTL条目信息
type TTLEntry struct {
	Key       string    `json:"key"`
	Expiry    time.Time `json:"expiry"`
	CreatedAt time.Time `json:"created_at"`
	TTL       time.Duration `json:"ttl"`
}

// IsExpired 检查TTL条目是否过期
func (e *TTLEntry) IsExpired() bool {
	return time.Now().After(e.Expiry)
}

// RemainingTTL 返回剩余的TTL时间
func (e *TTLEntry) RemainingTTL() time.Duration {
	remaining := time.Until(e.Expiry)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// TTLManager TTL管理器
type TTLManager struct {
	mu               sync.RWMutex
	entries          map[string]*TTLEntry
	defaultTTL       time.Duration
	maxTTL           time.Duration
	minTTL           time.Duration
	expiredCallback  func(key string)
	updateCallback   func(key string, oldTTL, newTTL time.Duration)
	contextCancel    context.CancelFunc
	contextDone      chan struct{}
}

// TTLConfig TTL管理器配置
type TTLConfig struct {
	DefaultTTL      time.Duration                                   `json:"default_ttl"`
	MaxTTL          time.Duration                                   `json:"max_ttl"`
	MinTTL          time.Duration                                   `json:"min_ttl"`
	ExpiredCallback func(key string)                                `json:"-"`
	UpdateCallback  func(key string, oldTTL, newTTL time.Duration) `json:"-"`
}

// DefaultTTLConfig 返回默认TTL配置
func DefaultTTLConfig() *TTLConfig {
	return &TTLConfig{
		DefaultTTL: 15 * time.Minute,
		MaxTTL:     24 * time.Hour,
		MinTTL:     1 * time.Minute,
	}
}

// NewTTLManager 创建新的TTL管理器
func NewTTLManager(config *TTLConfig) *TTLManager {
	if config == nil {
		config = DefaultTTLConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())
	
	tm := &TTLManager{
		entries:         make(map[string]*TTLEntry),
		defaultTTL:      config.DefaultTTL,
		maxTTL:          config.MaxTTL,
		minTTL:          config.MinTTL,
		expiredCallback: config.ExpiredCallback,
		updateCallback:  config.UpdateCallback,
		contextCancel:   cancel,
		contextDone:     make(chan struct{}),
	}

	// 启动TTL监控协程
	go tm.monitorTTL(ctx)

	return tm
}

// Set 设置键的TTL
func (tm *TTLManager) Set(key string, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = tm.defaultTTL
	}

	// 验证TTL范围
	if ttl < tm.minTTL {
		ttl = tm.minTTL
	}
	if ttl > tm.maxTTL {
		ttl = tm.maxTTL
	}

	now := time.Now()
	
	tm.mu.Lock()
	defer tm.mu.Unlock()

	oldEntry, exists := tm.entries[key]
	
	entry := &TTLEntry{
		Key:       key,
		Expiry:    now.Add(ttl),
		CreatedAt: now,
		TTL:       ttl,
	}

	tm.entries[key] = entry

	// 调用更新回调
	if tm.updateCallback != nil && exists {
		go tm.updateCallback(key, oldEntry.TTL, ttl)
	}

	return nil
}

// Get 获取键的TTL信息
func (tm *TTLManager) Get(key string) (*TTLEntry, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	entry, exists := tm.entries[key]
	if !exists {
		return nil, false
	}

	// 返回副本，防止外部修改
	entryCopy := *entry
	return &entryCopy, true
}

// GetRemainingTTL 获取键的剩余TTL时间
func (tm *TTLManager) GetRemainingTTL(key string) (time.Duration, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	entry, exists := tm.entries[key]
	if !exists {
		return 0, false
	}

	if entry.IsExpired() {
		return 0, false
	}

	return entry.RemainingTTL(), true
}

// Extend 延长键的TTL时间
func (tm *TTLManager) Extend(key string, additionalTTL time.Duration) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	entry, exists := tm.entries[key]
	if !exists {
		return &TTLError{
			Op:  "extend",
			Key: key,
			Err: "key not found",
		}
	}

	if entry.IsExpired() {
		delete(tm.entries, key)
		return &TTLError{
			Op:  "extend",
			Key: key,
			Err: "key has expired",
		}
	}

	oldTTL := entry.TTL
	newExpiry := entry.Expiry.Add(additionalTTL)
	newTTL := time.Until(newExpiry)

	// 验证新的TTL不超过最大值
	if newTTL > tm.maxTTL {
		newExpiry = time.Now().Add(tm.maxTTL)
		newTTL = tm.maxTTL
	}

	entry.Expiry = newExpiry
	entry.TTL = newTTL

	// 调用更新回调
	if tm.updateCallback != nil {
		go tm.updateCallback(key, oldTTL, newTTL)
	}

	return nil
}

// Refresh 刷新键的TTL，重置为默认或指定时间
func (tm *TTLManager) Refresh(key string, ttl ...time.Duration) error {
	var newTTL time.Duration
	if len(ttl) > 0 && ttl[0] > 0 {
		newTTL = ttl[0]
	} else {
		newTTL = tm.defaultTTL
	}

	return tm.Set(key, newTTL)
}

// Delete 删除键的TTL管理
func (tm *TTLManager) Delete(key string) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	_, exists := tm.entries[key]
	if exists {
		delete(tm.entries, key)
		return true
	}

	return false
}

// IsExpired 检查键是否已过期
func (tm *TTLManager) IsExpired(key string) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	entry, exists := tm.entries[key]
	if !exists {
		return true // 不存在的键视为已过期
	}

	return entry.IsExpired()
}

// GetExpiredKeys 获取所有已过期的键
func (tm *TTLManager) GetExpiredKeys() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var expiredKeys []string
	now := time.Now()

	for key, entry := range tm.entries {
		if now.After(entry.Expiry) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	return expiredKeys
}

// CleanupExpired 清理所有过期的键
func (tm *TTLManager) CleanupExpired() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	var expiredKeys []string
	now := time.Now()

	// 查找过期键
	for key, entry := range tm.entries {
		if now.After(entry.Expiry) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	// 删除过期键并触发回调
	for _, key := range expiredKeys {
		delete(tm.entries, key)
		if tm.expiredCallback != nil {
			go tm.expiredCallback(key)
		}
	}

	return len(expiredKeys)
}

// GetStats 获取TTL管理器统计信息
func (tm *TTLManager) GetStats() map[string]interface{} {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	totalEntries := len(tm.entries)
	expiredCount := 0
	var totalTTL time.Duration
	var avgTTL time.Duration
	
	now := time.Now()
	
	for _, entry := range tm.entries {
		totalTTL += entry.TTL
		if now.After(entry.Expiry) {
			expiredCount++
		}
	}

	if totalEntries > 0 {
		avgTTL = totalTTL / time.Duration(totalEntries)
	}

	return map[string]interface{}{
		"total_entries":   totalEntries,
		"expired_entries": expiredCount,
		"active_entries":  totalEntries - expiredCount,
		"default_ttl":     tm.defaultTTL.String(),
		"max_ttl":         tm.maxTTL.String(),
		"min_ttl":         tm.minTTL.String(),
		"avg_ttl":         avgTTL.String(),
	}
}

// GetAllEntries 获取所有TTL条目（用于调试）
func (tm *TTLManager) GetAllEntries() map[string]*TTLEntry {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	entries := make(map[string]*TTLEntry)
	for key, entry := range tm.entries {
		// 返回副本
		entryCopy := *entry
		entries[key] = &entryCopy
	}

	return entries
}

// SetDefaultTTL 设置默认TTL
func (tm *TTLManager) SetDefaultTTL(ttl time.Duration) {
	if ttl < tm.minTTL {
		ttl = tm.minTTL
	}
	if ttl > tm.maxTTL {
		ttl = tm.maxTTL
	}

	tm.mu.Lock()
	tm.defaultTTL = ttl
	tm.mu.Unlock()
}

// GetDefaultTTL 获取默认TTL
func (tm *TTLManager) GetDefaultTTL() time.Duration {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.defaultTTL
}

// monitorTTL TTL监控协程
func (tm *TTLManager) monitorTTL(ctx context.Context) {
	defer close(tm.contextDone)
	
	ticker := time.NewTicker(1 * time.Minute) // 每分钟检查一次
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tm.CleanupExpired()
		}
	}
}

// Close 关闭TTL管理器
func (tm *TTLManager) Close() error {
	if tm.contextCancel != nil {
		tm.contextCancel()
	}

	// 等待监控协程结束
	select {
	case <-tm.contextDone:
	case <-time.After(5 * time.Second):
		// 超时
	}

	// 清理所有条目
	tm.mu.Lock()
	tm.entries = make(map[string]*TTLEntry)
	tm.mu.Unlock()

	return nil
}

// Size 返回TTL条目数量
func (tm *TTLManager) Size() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.entries)
}

// TTLError TTL操作错误
type TTLError struct {
	Op  string
	Key string
	Err string
}

func (e *TTLError) Error() string {
	return "ttl " + e.Op + " " + e.Key + ": " + e.Err
}