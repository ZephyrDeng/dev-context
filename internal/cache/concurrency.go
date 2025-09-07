package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// ConcurrencyLimiter 并发限制器接口
type ConcurrencyLimiter interface {
	// Acquire 获取并发槽位
	Acquire(ctx context.Context) error
	// Release 释放并发槽位
	Release()
	// TryAcquire 尝试获取槽位，不阻塞
	TryAcquire() bool
	// Available 返回可用槽位数
	Available() int
	// InUse 返回正在使用的槽位数
	InUse() int
	// Close 关闭限制器
	Close() error
}

// SemaphoreLimiter 信号量并发限制器
type SemaphoreLimiter struct {
	semaphore chan struct{}
	capacity  int
	inUse     int64
	closed    int64
}

// NewSemaphoreLimiter 创建信号量并发限制器
func NewSemaphoreLimiter(capacity int) *SemaphoreLimiter {
	if capacity <= 0 {
		capacity = 100 // 默认100个并发槽位
	}
	
	return &SemaphoreLimiter{
		semaphore: make(chan struct{}, capacity),
		capacity:  capacity,
	}
}

// Acquire 获取并发槽位
func (s *SemaphoreLimiter) Acquire(ctx context.Context) error {
	if atomic.LoadInt64(&s.closed) == 1 {
		return errors.New("limiter is closed")
	}
	
	select {
	case s.semaphore <- struct{}{}:
		atomic.AddInt64(&s.inUse, 1)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release 释放并发槽位
func (s *SemaphoreLimiter) Release() {
	if atomic.LoadInt64(&s.closed) == 1 {
		return
	}
	
	select {
	case <-s.semaphore:
		atomic.AddInt64(&s.inUse, -1)
	default:
		// 信号量已为空，可能是重复释放
	}
}

// TryAcquire 尝试获取槽位，不阻塞
func (s *SemaphoreLimiter) TryAcquire() bool {
	if atomic.LoadInt64(&s.closed) == 1 {
		return false
	}
	
	select {
	case s.semaphore <- struct{}{}:
		atomic.AddInt64(&s.inUse, 1)
		return true
	default:
		return false
	}
}

// Available 返回可用槽位数
func (s *SemaphoreLimiter) Available() int {
	return s.capacity - int(atomic.LoadInt64(&s.inUse))
}

// InUse 返回正在使用的槽位数
func (s *SemaphoreLimiter) InUse() int {
	return int(atomic.LoadInt64(&s.inUse))
}

// Close 关闭限制器
func (s *SemaphoreLimiter) Close() error {
	if !atomic.CompareAndSwapInt64(&s.closed, 0, 1) {
		return errors.New("limiter already closed")
	}
	
	close(s.semaphore)
	return nil
}

// WorkerPool 工作池并发控制器
type WorkerPool struct {
	workers   int
	taskQueue chan func()
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
	stats     WorkerPoolStats
}

// WorkerPoolStats 工作池统计信息
type WorkerPoolStats struct {
	mu             sync.RWMutex
	TasksSubmitted int64 `json:"tasks_submitted"` // 提交的任务数
	TasksCompleted int64 `json:"tasks_completed"` // 完成的任务数
	TasksFailed    int64 `json:"tasks_failed"`    // 失败的任务数
	ActiveTasks    int64 `json:"active_tasks"`    // 当前活跃的任务数
}

// GetStats 获取统计信息
func (s *WorkerPoolStats) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return map[string]interface{}{
		"tasks_submitted": s.TasksSubmitted,
		"tasks_completed": s.TasksCompleted,
		"tasks_failed":    s.TasksFailed,
		"active_tasks":    s.ActiveTasks,
		"queue_length":    s.TasksSubmitted - s.TasksCompleted - s.TasksFailed,
	}
}

// NewWorkerPool 创建工作池
func NewWorkerPool(workers int, queueSize int) *WorkerPool {
	if workers <= 0 {
		workers = 10 // 默认10个工作协程
	}
	if queueSize <= 0 {
		queueSize = 1000 // 默认1000的队列大小
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	wp := &WorkerPool{
		workers:   workers,
		taskQueue: make(chan func(), queueSize),
		ctx:       ctx,
		cancel:    cancel,
	}
	
	// 启动工作协程
	for i := 0; i < workers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
	
	return wp
}

// worker 工作协程
func (wp *WorkerPool) worker() {
	defer wp.wg.Done()
	
	for {
		select {
		case task, ok := <-wp.taskQueue:
			if !ok {
				return // 任务队列已关闭
			}
			
			wp.stats.mu.Lock()
			wp.stats.ActiveTasks++
			wp.stats.mu.Unlock()
			
			// 执行任务
			func() {
				defer func() {
					wp.stats.mu.Lock()
					wp.stats.ActiveTasks--
					wp.stats.mu.Unlock()
					
					if r := recover(); r != nil {
						wp.stats.mu.Lock()
						wp.stats.TasksFailed++
						wp.stats.mu.Unlock()
					} else {
						wp.stats.mu.Lock()
						wp.stats.TasksCompleted++
						wp.stats.mu.Unlock()
					}
				}()
				
				task()
			}()
			
		case <-wp.ctx.Done():
			return
		}
	}
}

// Submit 提交任务到工作池
func (wp *WorkerPool) Submit(task func()) error {
	if wp.ctx.Err() != nil {
		return errors.New("worker pool is closed")
	}
	
	wp.stats.mu.Lock()
	wp.stats.TasksSubmitted++
	wp.stats.mu.Unlock()
	
	select {
	case wp.taskQueue <- task:
		return nil
	default:
		wp.stats.mu.Lock()
		wp.stats.TasksFailed++
		wp.stats.mu.Unlock()
		return errors.New("task queue is full")
	}
}

// TrySubmit 尝试提交任务，不阻塞
func (wp *WorkerPool) TrySubmit(task func()) bool {
	if wp.ctx.Err() != nil {
		return false
	}
	
	select {
	case wp.taskQueue <- task:
		wp.stats.mu.Lock()
		wp.stats.TasksSubmitted++
		wp.stats.mu.Unlock()
		return true
	default:
		return false
	}
}

// SubmitWithTimeout 在指定超时时间内提交任务
func (wp *WorkerPool) SubmitWithTimeout(task func(), timeout time.Duration) error {
	if wp.ctx.Err() != nil {
		return errors.New("worker pool is closed")
	}
	
	wp.stats.mu.Lock()
	wp.stats.TasksSubmitted++
	wp.stats.mu.Unlock()
	
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	
	select {
	case wp.taskQueue <- task:
		return nil
	case <-timer.C:
		wp.stats.mu.Lock()
		wp.stats.TasksFailed++
		wp.stats.mu.Unlock()
		return errors.New("submit timeout")
	case <-wp.ctx.Done():
		wp.stats.mu.Lock()
		wp.stats.TasksFailed++
		wp.stats.mu.Unlock()
		return errors.New("worker pool is closed")
	}
}

// QueueLength 返回队列长度
func (wp *WorkerPool) QueueLength() int {
	return len(wp.taskQueue)
}

// GetStats 获取工作池统计信息
func (wp *WorkerPool) GetStats() map[string]interface{} {
	stats := wp.stats.GetStats()
	stats["workers"] = wp.workers
	stats["queue_capacity"] = cap(wp.taskQueue)
	stats["queue_length"] = wp.QueueLength()
	return stats
}

// Close 关闭工作池
func (wp *WorkerPool) Close() error {
	wp.cancel()
	close(wp.taskQueue)
	wp.wg.Wait()
	return nil
}

// RateLimiter 速率限制器
type RateLimiter struct {
	ticker   *time.Ticker
	tokens   chan struct{}
	rate     time.Duration
	burst    int
	closed   bool
	closeMu  sync.Mutex
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(rate time.Duration, burst int) *RateLimiter {
	if burst <= 0 {
		burst = 1
	}
	
	rl := &RateLimiter{
		ticker: time.NewTicker(rate),
		tokens: make(chan struct{}, burst),
		rate:   rate,
		burst:  burst,
	}
	
	// 初始填充令牌桶
	for i := 0; i < burst; i++ {
		select {
		case rl.tokens <- struct{}{}:
		default:
		}
	}
	
	// 启动令牌生成协程
	go rl.generateTokens()
	
	return rl
}

// generateTokens 生成令牌
func (rl *RateLimiter) generateTokens() {
	defer rl.ticker.Stop()
	
	for range rl.ticker.C {
		rl.closeMu.Lock()
		if rl.closed {
			rl.closeMu.Unlock()
			return
		}
		rl.closeMu.Unlock()
		
		select {
		case rl.tokens <- struct{}{}:
		default:
			// 令牌桶已满
		}
	}
}

// Wait 等待获取令牌
func (rl *RateLimiter) Wait(ctx context.Context) error {
	rl.closeMu.Lock()
	if rl.closed {
		rl.closeMu.Unlock()
		return errors.New("rate limiter is closed")
	}
	rl.closeMu.Unlock()
	
	select {
	case <-rl.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TryWait 尝试获取令牌，不阻塞
func (rl *RateLimiter) TryWait() bool {
	rl.closeMu.Lock()
	if rl.closed {
		rl.closeMu.Unlock()
		return false
	}
	rl.closeMu.Unlock()
	
	select {
	case <-rl.tokens:
		return true
	default:
		return false
	}
}

// Available 返回可用令牌数
func (rl *RateLimiter) Available() int {
	return len(rl.tokens)
}

// Close 关闭速率限制器
func (rl *RateLimiter) Close() error {
	rl.closeMu.Lock()
	defer rl.closeMu.Unlock()
	
	if rl.closed {
		return errors.New("rate limiter already closed")
	}
	
	rl.closed = true
	rl.ticker.Stop()
	close(rl.tokens)
	return nil
}

// ConcurrencyManager 并发管理器
type ConcurrencyManager struct {
	limiter    ConcurrencyLimiter
	workerPool *WorkerPool
	rateLimiter *RateLimiter
	stats      ConcurrencyStats
}

// ConcurrencyStats 并发管理统计
type ConcurrencyStats struct {
	mu                sync.RWMutex
	ConcurrentRequests int64 `json:"concurrent_requests"` // 当前并发请求数
	TotalRequests     int64 `json:"total_requests"`      // 总请求数
	RejectedRequests  int64 `json:"rejected_requests"`   // 被拒绝的请求数
	AverageWaitTime   int64 `json:"average_wait_time"`   // 平均等待时间（毫秒）
}

// ConcurrencyConfig 并发管理配置
type ConcurrencyConfig struct {
	MaxConcurrency int           `json:"max_concurrency"` // 最大并发数
	WorkerPoolSize int           `json:"worker_pool_size"` // 工作池大小
	QueueSize      int           `json:"queue_size"`       // 队列大小
	RateLimit      time.Duration `json:"rate_limit"`       // 速率限制间隔
	BurstLimit     int           `json:"burst_limit"`      // 突发限制
}

// DefaultConcurrencyConfig 默认并发配置
func DefaultConcurrencyConfig() *ConcurrencyConfig {
	return &ConcurrencyConfig{
		MaxConcurrency: 100,
		WorkerPoolSize: 10,
		QueueSize:      1000,
		RateLimit:      10 * time.Millisecond, // 每10毫秒一个令牌
		BurstLimit:     50,
	}
}

// NewConcurrencyManager 创建并发管理器
func NewConcurrencyManager(config *ConcurrencyConfig) *ConcurrencyManager {
	if config == nil {
		config = DefaultConcurrencyConfig()
	}
	
	return &ConcurrencyManager{
		limiter:     NewSemaphoreLimiter(config.MaxConcurrency),
		workerPool:  NewWorkerPool(config.WorkerPoolSize, config.QueueSize),
		rateLimiter: NewRateLimiter(config.RateLimit, config.BurstLimit),
	}
}

// ExecuteWithLimits 在并发限制下执行任务
func (cm *ConcurrencyManager) ExecuteWithLimits(ctx context.Context, task func() error) error {
	start := time.Now()
	
	// 速率限制
	if err := cm.rateLimiter.Wait(ctx); err != nil {
		cm.stats.mu.Lock()
		cm.stats.RejectedRequests++
		cm.stats.mu.Unlock()
		return err
	}
	
	// 并发限制
	if err := cm.limiter.Acquire(ctx); err != nil {
		cm.stats.mu.Lock()
		cm.stats.RejectedRequests++
		cm.stats.mu.Unlock()
		return err
	}
	defer cm.limiter.Release()
	
	// 更新统计
	cm.stats.mu.Lock()
	cm.stats.TotalRequests++
	cm.stats.ConcurrentRequests++
	waitTime := time.Since(start).Milliseconds()
	
	// 简单的滑动平均计算
	if cm.stats.AverageWaitTime == 0 {
		cm.stats.AverageWaitTime = waitTime
	} else {
		cm.stats.AverageWaitTime = (cm.stats.AverageWaitTime*9 + waitTime) / 10
	}
	cm.stats.mu.Unlock()
	
	defer func() {
		cm.stats.mu.Lock()
		cm.stats.ConcurrentRequests--
		cm.stats.mu.Unlock()
	}()
	
	return task()
}

// SubmitToWorkerPool 提交任务到工作池
func (cm *ConcurrencyManager) SubmitToWorkerPool(task func()) error {
	return cm.workerPool.Submit(task)
}

// GetStats 获取并发管理统计信息
func (cm *ConcurrencyManager) GetStats() map[string]interface{} {
	cm.stats.mu.RLock()
	defer cm.stats.mu.RUnlock()
	
	stats := map[string]interface{}{
		"concurrent_requests": cm.stats.ConcurrentRequests,
		"total_requests":      cm.stats.TotalRequests,
		"rejected_requests":   cm.stats.RejectedRequests,
		"average_wait_time":   cm.stats.AverageWaitTime,
		"available_slots":     cm.limiter.Available(),
		"slots_in_use":        cm.limiter.InUse(),
		"rate_tokens":         cm.rateLimiter.Available(),
	}
	
	// 合并工作池统计
	workerStats := cm.workerPool.GetStats()
	for k, v := range workerStats {
		stats["worker_pool_"+k] = v
	}
	
	return stats
}

// Close 关闭并发管理器
func (cm *ConcurrencyManager) Close() error {
	var errs []error
	
	if err := cm.limiter.Close(); err != nil {
		errs = append(errs, err)
	}
	
	if err := cm.workerPool.Close(); err != nil {
		errs = append(errs, err)
	}
	
	if err := cm.rateLimiter.Close(); err != nil {
		errs = append(errs, err)
	}
	
	if len(errs) > 0 {
		return errors.New("multiple close errors occurred")
	}
	
	return nil
}