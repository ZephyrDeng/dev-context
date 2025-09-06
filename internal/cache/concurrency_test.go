package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// SemaphoreLimiter 测试

func TestSemaphoreLimiterBasic(t *testing.T) {
	capacity := 3
	limiter := NewSemaphoreLimiter(capacity)
	defer limiter.Close()
	
	ctx := context.Background()
	
	// 初始状态检查
	if limiter.Available() != capacity {
		t.Errorf("Expected %d available slots, got %d", capacity, limiter.Available())
	}
	
	if limiter.InUse() != 0 {
		t.Errorf("Expected 0 slots in use, got %d", limiter.InUse())
	}
	
	// 获取所有槽位
	for i := 0; i < capacity; i++ {
		err := limiter.Acquire(ctx)
		if err != nil {
			t.Fatalf("Unexpected error acquiring slot %d: %v", i, err)
		}
		
		expectedAvailable := capacity - (i + 1)
		expectedInUse := i + 1
		
		if limiter.Available() != expectedAvailable {
			t.Errorf("After acquiring slot %d, expected %d available, got %d", i, expectedAvailable, limiter.Available())
		}
		
		if limiter.InUse() != expectedInUse {
			t.Errorf("After acquiring slot %d, expected %d in use, got %d", i, expectedInUse, limiter.InUse())
		}
	}
	
	// 释放所有槽位
	for i := 0; i < capacity; i++ {
		limiter.Release()
		
		expectedAvailable := i + 1
		expectedInUse := capacity - (i + 1)
		
		if limiter.Available() != expectedAvailable {
			t.Errorf("After releasing slot %d, expected %d available, got %d", i, expectedAvailable, limiter.Available())
		}
		
		if limiter.InUse() != expectedInUse {
			t.Errorf("After releasing slot %d, expected %d in use, got %d", i, expectedInUse, limiter.InUse())
		}
	}
}

func TestSemaphoreLimiterConcurrent(t *testing.T) {
	capacity := 5
	limiter := NewSemaphoreLimiter(capacity)
	defer limiter.Close()
	
	ctx := context.Background()
	goroutines := 20
	acquired := int32(0)
	maxConcurrent := int32(0)
	
	var wg sync.WaitGroup
	
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			err := limiter.Acquire(ctx)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			defer limiter.Release()
			
			current := atomic.AddInt32(&acquired, 1)
			
			// 更新最大并发数
			for {
				max := atomic.LoadInt32(&maxConcurrent)
				if current <= max || atomic.CompareAndSwapInt32(&maxConcurrent, max, current) {
					break
				}
			}
			
			// 模拟工作
			time.Sleep(50 * time.Millisecond)
			
			atomic.AddInt32(&acquired, -1)
		}()
	}
	
	wg.Wait()
	
	// 验证最大并发数不超过容量
	if maxConcurrent > int32(capacity) {
		t.Errorf("Max concurrent %d exceeded capacity %d", maxConcurrent, capacity)
	}
	
	// 最终状态检查
	if limiter.InUse() != 0 {
		t.Errorf("Expected 0 slots in use after completion, got %d", limiter.InUse())
	}
	
	if limiter.Available() != capacity {
		t.Errorf("Expected %d available slots after completion, got %d", capacity, limiter.Available())
	}
}

func TestSemaphoreLimiterTimeout(t *testing.T) {
	capacity := 1
	limiter := NewSemaphoreLimiter(capacity)
	defer limiter.Close()
	
	ctx := context.Background()
	
	// 获取唯一的槽位
	err := limiter.Acquire(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	// 尝试用超时上下文获取槽位
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	start := time.Now()
	err = limiter.Acquire(timeoutCtx)
	duration := time.Since(start)
	
	if err == nil {
		t.Error("Expected timeout error")
	}
	
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", err)
	}
	
	// 验证确实等待了接近超时时间
	if duration < 90*time.Millisecond || duration > 200*time.Millisecond {
		t.Errorf("Expected duration around 100ms, got %v", duration)
	}
	
	// 释放槽位
	limiter.Release()
}

func TestSemaphoreLimiterTryAcquire(t *testing.T) {
	capacity := 2
	limiter := NewSemaphoreLimiter(capacity)
	defer limiter.Close()
	
	// 应该能成功获取槽位
	if !limiter.TryAcquire() {
		t.Error("Expected successful try acquire")
	}
	
	if !limiter.TryAcquire() {
		t.Error("Expected successful try acquire")
	}
	
	// 现在应该满了
	if limiter.TryAcquire() {
		t.Error("Expected failed try acquire when full")
	}
	
	// 释放一个槽位
	limiter.Release()
	
	// 现在应该能再次获取
	if !limiter.TryAcquire() {
		t.Error("Expected successful try acquire after release")
	}
}

func TestSemaphoreLimiterClose(t *testing.T) {
	limiter := NewSemaphoreLimiter(1)
	
	ctx := context.Background()
	
	// 正常获取
	err := limiter.Acquire(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	// 关闭限制器
	err = limiter.Close()
	if err != nil {
		t.Errorf("Unexpected close error: %v", err)
	}
	
	// 关闭后的操作应该失败
	err = limiter.Acquire(ctx)
	if err == nil {
		t.Error("Expected error after close")
	}
	
	if limiter.TryAcquire() {
		t.Error("Expected try acquire to fail after close")
	}
	
	// 重复关闭
	err = limiter.Close()
	if err == nil {
		t.Error("Expected error on double close")
	}
}

// WorkerPool 测试

func TestWorkerPoolBasic(t *testing.T) {
	workers := 3
	queueSize := 10
	pool := NewWorkerPool(workers, queueSize)
	defer pool.Close()
	
	counter := int32(0)
	tasks := 5
	
	var wg sync.WaitGroup
	for i := 0; i < tasks; i++ {
		wg.Add(1)
		err := pool.Submit(func() {
			defer wg.Done()
			atomic.AddInt32(&counter, 1)
			time.Sleep(10 * time.Millisecond)
		})
		
		if err != nil {
			t.Errorf("Unexpected submit error: %v", err)
			wg.Done()
		}
	}
	
	wg.Wait()
	
	if atomic.LoadInt32(&counter) != int32(tasks) {
		t.Errorf("Expected %d tasks executed, got %d", tasks, counter)
	}
	
	// 检查统计
	stats := pool.GetStats()
	submitted := stats["tasks_submitted"].(int64)
	completed := stats["tasks_completed"].(int64)
	
	if submitted != int64(tasks) {
		t.Errorf("Expected %d tasks submitted, got %d", tasks, submitted)
	}
	
	if completed != int64(tasks) {
		t.Errorf("Expected %d tasks completed, got %d", tasks, completed)
	}
}

func TestWorkerPoolFullQueue(t *testing.T) {
	workers := 1
	queueSize := 2
	pool := NewWorkerPool(workers, queueSize)
	defer pool.Close()
	
	// 提交阻塞任务填满队列
	blockChan := make(chan struct{})
	
	// 第一个任务会被工作协程执行并阻塞
	err := pool.Submit(func() {
		<-blockChan
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	time.Sleep(10 * time.Millisecond) // 让第一个任务开始执行
	
	// 接下来的任务会填满队列
	for i := 0; i < queueSize; i++ {
		err := pool.Submit(func() {
			time.Sleep(1 * time.Millisecond)
		})
		if err != nil {
			t.Errorf("Unexpected error submitting task %d: %v", i, err)
		}
	}
	
	// 现在队列应该满了，下一个提交应该失败
	err = pool.Submit(func() {})
	if err == nil {
		t.Error("Expected error when queue is full")
	}
	
	// 解除阻塞
	close(blockChan)
	
	// 等待一点时间让任务完成
	time.Sleep(50 * time.Millisecond)
}

func TestWorkerPoolTrySubmit(t *testing.T) {
	workers := 1
	queueSize := 1
	pool := NewWorkerPool(workers, queueSize)
	defer pool.Close()
	
	// 第一个任务应该成功
	success := pool.TrySubmit(func() {
		time.Sleep(100 * time.Millisecond)
	})
	if !success {
		t.Error("Expected first try submit to succeed")
	}
	
	time.Sleep(10 * time.Millisecond) // 让任务开始执行
	
	// 第二个任务也应该成功（进入队列）
	success = pool.TrySubmit(func() {
		time.Sleep(1 * time.Millisecond)
	})
	if !success {
		t.Error("Expected second try submit to succeed")
	}
	
	// 第三个任务应该失败（队列满）
	success = pool.TrySubmit(func() {})
	if success {
		t.Error("Expected third try submit to fail")
	}
}

func TestWorkerPoolWithTimeout(t *testing.T) {
	workers := 1
	queueSize := 1
	pool := NewWorkerPool(workers, queueSize)
	defer pool.Close()
	
	// 提交阻塞任务
	blockChan := make(chan struct{})
	err := pool.Submit(func() {
		<-blockChan
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	time.Sleep(10 * time.Millisecond) // 让任务开始执行
	
	// 填满队列
	err = pool.Submit(func() {})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	// 超时提交应该失败
	err = pool.SubmitWithTimeout(func() {}, 50*time.Millisecond)
	if err == nil {
		t.Error("Expected timeout error")
	}
	
	if err.Error() != "submit timeout" {
		t.Errorf("Expected timeout error, got %v", err)
	}
	
	close(blockChan)
}

func TestWorkerPoolPanic(t *testing.T) {
	pool := NewWorkerPool(1, 10)
	defer pool.Close()
	
	// 提交会panic的任务
	err := pool.Submit(func() {
		panic("test panic")
	})
	if err != nil {
		t.Fatalf("Unexpected submit error: %v", err)
	}
	
	// 提交正常任务
	done := make(chan struct{})
	err = pool.Submit(func() {
		close(done)
	})
	if err != nil {
		t.Fatalf("Unexpected submit error: %v", err)
	}
	
	// 等待正常任务完成
	select {
	case <-done:
		// OK
	case <-time.After(1 * time.Second):
		t.Error("Worker pool should continue working after panic")
	}
	
	// 检查统计
	time.Sleep(100 * time.Millisecond)
	stats := pool.GetStats()
	failed := stats["tasks_failed"].(int64)
	completed := stats["tasks_completed"].(int64)
	
	if failed != 1 {
		t.Errorf("Expected 1 failed task, got %d", failed)
	}
	
	if completed != 1 {
		t.Errorf("Expected 1 completed task, got %d", completed)
	}
}

// RateLimiter 测试

func TestRateLimiterBasic(t *testing.T) {
	rate := 100 * time.Millisecond
	burst := 3
	limiter := NewRateLimiter(rate, burst)
	defer limiter.Close()
	
	ctx := context.Background()
	
	// 初始应该有burst个令牌
	available := limiter.Available()
	if available != burst {
		t.Errorf("Expected %d initial tokens, got %d", burst, available)
	}
	
	// 快速消费所有初始令牌
	for i := 0; i < burst; i++ {
		err := limiter.Wait(ctx)
		if err != nil {
			t.Errorf("Unexpected error consuming token %d: %v", i, err)
		}
	}
	
	// 现在应该没有令牌了
	if limiter.Available() != 0 {
		t.Errorf("Expected 0 tokens after consuming burst, got %d", limiter.Available())
	}
	
	// 下一次等待应该被延迟
	start := time.Now()
	err := limiter.Wait(ctx)
	duration := time.Since(start)
	
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	// 应该等待了接近rate的时间
	if duration < 80*time.Millisecond || duration > 150*time.Millisecond {
		t.Errorf("Expected duration around %v, got %v", rate, duration)
	}
}

func TestRateLimiterTryWait(t *testing.T) {
	rate := 100 * time.Millisecond
	burst := 2
	limiter := NewRateLimiter(rate, burst)
	defer limiter.Close()
	
	// 应该能立即获取burst个令牌
	for i := 0; i < burst; i++ {
		if !limiter.TryWait() {
			t.Errorf("Expected successful try wait %d", i)
		}
	}
	
	// 现在应该失败
	if limiter.TryWait() {
		t.Error("Expected try wait to fail when no tokens")
	}
	
	// 等待令牌生成
	time.Sleep(150 * time.Millisecond)
	
	// 现在应该有新令牌
	if !limiter.TryWait() {
		t.Error("Expected try wait to succeed after token generation")
	}
}

func TestRateLimiterContext(t *testing.T) {
	rate := 1 * time.Second // 很长的间隔
	burst := 1
	limiter := NewRateLimiter(rate, burst)
	defer limiter.Close()
	
	ctx := context.Background()
	
	// 消费初始令牌
	err := limiter.Wait(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	// 用取消的上下文等待
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消
	
	err = limiter.Wait(cancelCtx)
	if err == nil {
		t.Error("Expected context cancellation error")
	}
	
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestRateLimiterClose(t *testing.T) {
	limiter := NewRateLimiter(100*time.Millisecond, 1)
	
	ctx := context.Background()
	
	// 正常使用
	err := limiter.Wait(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	// 关闭
	err = limiter.Close()
	if err != nil {
		t.Errorf("Unexpected close error: %v", err)
	}
	
	// 关闭后的操作应该失败
	err = limiter.Wait(ctx)
	if err == nil {
		t.Error("Expected error after close")
	}
	
	if limiter.TryWait() {
		t.Error("Expected try wait to fail after close")
	}
	
	// 重复关闭
	err = limiter.Close()
	if err == nil {
		t.Error("Expected error on double close")
	}
}

// ConcurrencyManager 测试

func TestConcurrencyManagerBasic(t *testing.T) {
	config := &ConcurrencyConfig{
		MaxConcurrency: 2,
		WorkerPoolSize: 2,
		QueueSize:      10,
		RateLimit:      10 * time.Millisecond,
		BurstLimit:     5,
	}
	
	manager := NewConcurrencyManager(config)
	defer manager.Close()
	
	ctx := context.Background()
	counter := int32(0)
	maxConcurrent := int32(0)
	current := int32(0)
	
	tasks := 10
	var wg sync.WaitGroup
	
	for i := 0; i < tasks; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			err := manager.ExecuteWithLimits(ctx, func() error {
				atomic.AddInt32(&counter, 1)
				
				// 跟踪最大并发
				curr := atomic.AddInt32(&current, 1)
				for {
					max := atomic.LoadInt32(&maxConcurrent)
					if curr <= max || atomic.CompareAndSwapInt32(&maxConcurrent, max, curr) {
						break
					}
				}
				
				time.Sleep(50 * time.Millisecond)
				atomic.AddInt32(&current, -1)
				return nil
			})
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		}()
	}
	
	wg.Wait()
	
	if atomic.LoadInt32(&counter) != int32(tasks) {
		t.Errorf("Expected %d tasks executed, got %d", tasks, counter)
	}
	
	// 验证并发限制
	if maxConcurrent > int32(config.MaxConcurrency) {
		t.Errorf("Max concurrent %d exceeded limit %d", maxConcurrent, config.MaxConcurrency)
	}
	
	// 检查统计
	stats := manager.GetStats()
	totalRequests := stats["total_requests"].(int64)
	
	if totalRequests != int64(tasks) {
		t.Errorf("Expected %d total requests, got %d", tasks, totalRequests)
	}
}

func TestConcurrencyManagerWorkerPool(t *testing.T) {
	manager := NewConcurrencyManager(nil)
	defer manager.Close()
	
	counter := int32(0)
	tasks := 5
	var wg sync.WaitGroup
	
	for i := 0; i < tasks; i++ {
		wg.Add(1)
		err := manager.SubmitToWorkerPool(func() {
			defer wg.Done()
			atomic.AddInt32(&counter, 1)
			time.Sleep(10 * time.Millisecond)
		})
		
		if err != nil {
			t.Errorf("Unexpected submit error: %v", err)
			wg.Done()
		}
	}
	
	wg.Wait()
	
	if atomic.LoadInt32(&counter) != int32(tasks) {
		t.Errorf("Expected %d tasks executed, got %d", tasks, counter)
	}
}

func TestConcurrencyManagerStats(t *testing.T) {
	manager := NewConcurrencyManager(nil)
	defer manager.Close()
	
	ctx := context.Background()
	
	// 执行一些任务
	for i := 0; i < 3; i++ {
		err := manager.ExecuteWithLimits(ctx, func() error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	}
	
	stats := manager.GetStats()
	
	// 验证基本统计
	if stats["total_requests"].(int64) != 3 {
		t.Errorf("Expected 3 total requests, got %d", stats["total_requests"].(int64))
	}
	
	if stats["rejected_requests"].(int64) != 0 {
		t.Errorf("Expected 0 rejected requests, got %d", stats["rejected_requests"].(int64))
	}
	
	// 验证包含工作池统计
	if _, exists := stats["worker_pool_workers"]; !exists {
		t.Error("Expected worker pool stats to be included")
	}
}

func TestConcurrencyManagerClose(t *testing.T) {
	manager := NewConcurrencyManager(nil)
	
	ctx := context.Background()
	
	// 正常执行
	err := manager.ExecuteWithLimits(ctx, func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	// 关闭管理器
	err = manager.Close()
	if err != nil {
		t.Errorf("Unexpected close error: %v", err)
	}
}

func TestConcurrencyManagerError(t *testing.T) {
	manager := NewConcurrencyManager(nil)
	defer manager.Close()
	
	ctx := context.Background()
	expectedError := errors.New("test error")
	
	err := manager.ExecuteWithLimits(ctx, func() error {
		return expectedError
	})
	
	if err == nil {
		t.Error("Expected task error to be returned")
	}
	
	if err != expectedError {
		t.Errorf("Expected %v, got %v", expectedError, err)
	}
}

func TestDefaultConcurrencyConfig(t *testing.T) {
	config := DefaultConcurrencyConfig()
	
	if config.MaxConcurrency != 100 {
		t.Errorf("Expected MaxConcurrency 100, got %d", config.MaxConcurrency)
	}
	
	if config.WorkerPoolSize != 10 {
		t.Errorf("Expected WorkerPoolSize 10, got %d", config.WorkerPoolSize)
	}
	
	if config.QueueSize != 1000 {
		t.Errorf("Expected QueueSize 1000, got %d", config.QueueSize)
	}
	
	if config.RateLimit != 10*time.Millisecond {
		t.Errorf("Expected RateLimit 10ms, got %v", config.RateLimit)
	}
	
	if config.BurstLimit != 50 {
		t.Errorf("Expected BurstLimit 50, got %d", config.BurstLimit)
	}
}