package cache

import (
	"crypto/md5"
	"fmt"
	"sync"
	"time"
)

// CachedResult 表示缓存的结果数据
type CachedResult struct {
	Data        interface{} `json:"data"`
	Timestamp   time.Time   `json:"timestamp"`
	Expiry      time.Time   `json:"expiry"`
	AccessCount int64       `json:"access_count"`
	Size        int64       `json:"size"`
}

// IsExpired 检查缓存项是否已过期
func (r *CachedResult) IsExpired() bool {
	return time.Now().After(r.Expiry)
}

// Touch 更新访问计数和时间戳
func (r *CachedResult) Touch() {
	r.AccessCount++
	r.Timestamp = time.Now()
}

// EstimateSize 估算缓存项的内存大小
func (r *CachedResult) EstimateSize() int64 {
	if r.Size > 0 {
		return r.Size
	}
	// 简单估算：基础结构大小 + 数据大小估算
	baseSize := int64(64) // 基础结构大小估算
	
	// 根据数据类型估算大小
	switch v := r.Data.(type) {
	case string:
		r.Size = baseSize + int64(len(v))
	case []byte:
		r.Size = baseSize + int64(len(v))
	case []interface{}:
		r.Size = baseSize + int64(len(v)*100) // 粗略估算
	default:
		r.Size = baseSize + 1024 // 默认1KB估算
	}
	
	return r.Size
}

// CacheStorage 提供线程安全的内存缓存存储
type CacheStorage struct {
	data      map[string]*CachedResult
	mutex     sync.RWMutex
	maxSize   int64
	totalSize int64
}

// NewCacheStorage 创建新的缓存存储实例
func NewCacheStorage(maxSize int64) *CacheStorage {
	if maxSize <= 0 {
		maxSize = 512 * 1024 * 1024 // 默认512MB
	}
	
	return &CacheStorage{
		data:    make(map[string]*CachedResult),
		maxSize: maxSize,
	}
}

// GenerateKey 生成缓存键
// 基于查询类型、参数和时间范围生成MD5哈希
func GenerateKey(queryType string, parameters map[string]interface{}, timeRange string) string {
	h := md5.New()
	
	// 查询类型
	h.Write([]byte(queryType))
	h.Write([]byte("|"))
	
	// 参数（按键排序确保一致性）
	if parameters != nil {
		// 先收集所有键并排序
		var keys []string
		for key := range parameters {
			keys = append(keys, key)
		}
		// 简单的字符串排序
		for i := 0; i < len(keys)-1; i++ {
			for j := i + 1; j < len(keys); j++ {
				if keys[i] > keys[j] {
					keys[i], keys[j] = keys[j], keys[i]
				}
			}
		}
		
		// 按排序后的键顺序写入
		for _, key := range keys {
			h.Write([]byte(key))
			h.Write([]byte("="))
			h.Write([]byte(fmt.Sprintf("%v", parameters[key])))
			h.Write([]byte("&"))
		}
	}
	
	// 时间范围
	h.Write([]byte("|"))
	h.Write([]byte(timeRange))
	
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Get 获取缓存项
func (s *CacheStorage) Get(key string) (*CachedResult, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	result, exists := s.data[key]
	if !exists {
		return nil, false
	}
	
	// 检查是否过期
	if result.IsExpired() {
		// 过期项在读取时清理
		s.mutex.RUnlock()
		s.mutex.Lock()
		delete(s.data, key)
		s.totalSize -= result.EstimateSize()
		s.mutex.Unlock()
		s.mutex.RLock()
		return nil, false
	}
	
	// 更新访问统计
	result.Touch()
	
	return result, true
}

// Set 设置缓存项
func (s *CacheStorage) Set(key string, data interface{}, ttl time.Duration) error {
	now := time.Now()
	result := &CachedResult{
		Data:        data,
		Timestamp:   now,
		Expiry:      now.Add(ttl),
		AccessCount: 1,
	}
	
	size := result.EstimateSize()
	
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	// 检查内存限制
	if s.totalSize+size > s.maxSize {
		// 尝试清理过期项和LRU清理
		s.cleanup()
		
		// 如果仍然超出限制，执行LRU清理
		if s.totalSize+size > s.maxSize {
			s.evictLRU(size)
		}
		
		// 如果单个项目超过最大大小，拒绝缓存
		if size > s.maxSize {
			return fmt.Errorf("cache item size %d exceeds maximum cache size %d", size, s.maxSize)
		}
	}
	
	// 如果键已存在，先删除旧项
	if oldResult, exists := s.data[key]; exists {
		s.totalSize -= oldResult.EstimateSize()
	}
	
	// 添加新项
	s.data[key] = result
	s.totalSize += size
	
	return nil
}

// Delete 删除缓存项
func (s *CacheStorage) Delete(key string) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	if result, exists := s.data[key]; exists {
		delete(s.data, key)
		s.totalSize -= result.EstimateSize()
		return true
	}
	
	return false
}

// Clear 清空所有缓存
func (s *CacheStorage) Clear() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.data = make(map[string]*CachedResult)
	s.totalSize = 0
}

// Size 返回缓存项数量
func (s *CacheStorage) Size() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	return len(s.data)
}

// TotalSize 返回缓存总大小
func (s *CacheStorage) TotalSize() int64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	return s.totalSize
}

// MaxSize 返回最大缓存大小
func (s *CacheStorage) MaxSize() int64 {
	return s.maxSize
}

// cleanup 清理过期项（调用时必须持有写锁）
func (s *CacheStorage) cleanup() {
	now := time.Now()
	for key, result := range s.data {
		if now.After(result.Expiry) {
			delete(s.data, key)
			s.totalSize -= result.EstimateSize()
		}
	}
}

// evictLRU 执行LRU清理（调用时必须持有写锁）
func (s *CacheStorage) evictLRU(neededSpace int64) {
	type lruItem struct {
		key       string
		timestamp time.Time
		size      int64
	}
	
	// 收集所有项目并按最后访问时间排序
	var items []lruItem
	for key, result := range s.data {
		items = append(items, lruItem{
			key:       key,
			timestamp: result.Timestamp,
			size:      result.EstimateSize(),
		})
	}
	
	// 简单的LRU排序：按时间戳升序
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].timestamp.After(items[j].timestamp) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	
	// 清理最旧的项目直到有足够空间
	freedSpace := int64(0)
	for _, item := range items {
		if s.totalSize-freedSpace+neededSpace <= s.maxSize {
			break
		}
		
		delete(s.data, item.key)
		s.totalSize -= item.size
		freedSpace += item.size
	}
}

// GetKeys 返回所有缓存键（用于调试和监控）
func (s *CacheStorage) GetKeys() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	keys := make([]string, 0, len(s.data))
	for key := range s.data {
		keys = append(keys, key)
	}
	
	return keys
}

// GetStats 获取缓存统计信息
func (s *CacheStorage) GetStats() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	totalAccess := int64(0)
	expiredCount := 0
	now := time.Now()
	
	for _, result := range s.data {
		totalAccess += result.AccessCount
		if now.After(result.Expiry) {
			expiredCount++
		}
	}
	
	return map[string]interface{}{
		"total_items":    len(s.data),
		"total_size":     s.totalSize,
		"max_size":       s.maxSize,
		"memory_usage":   float64(s.totalSize) / float64(s.maxSize) * 100,
		"total_accesses": totalAccess,
		"expired_items":  expiredCount,
	}
}