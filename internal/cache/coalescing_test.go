package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCoalescingResult(t *testing.T) {
	result := &CachedResult{Data: "test data"}
	coalescingResult := CoalescingResult{result: result, err: nil}
	
	if coalescingResult.result != result {
		t.Errorf("Expected result %v, got %v", result, coalescingResult.result)
	}
	
	if coalescingResult.err != nil {
		t.Errorf("Expected no error, got %v", coalescingResult.err)
	}
}

func TestCoalescingGroup(t *testing.T) {
	timeout := 5 * time.Second
	group := NewCoalescingGroup(timeout)
	
	if group.count != 1 {
		t.Errorf("Expected count 1, got %d", group.count)
	}
	
	if group.timeout != timeout {
		t.Errorf("Expected timeout %v, got %v", timeout, group.timeout)
	}
	
	if group.IsExpired() {
		t.Error("New group should not be expired")
	}
	
	// 测试过期检查
	group.created = time.Now().Add(-10 * time.Second)
	if !group.IsExpired() {
		t.Error("Group should be expired")
	}
}

func TestQueryCoalescerBasic(t *testing.T) {
	config := &CoalescingConfig{
		Timeout:      5 * time.Second,
		CleanupDelay: 1 * time.Second,
	}
	coalescer := NewQueryCoalescer(config)
	defer coalescer.Close()
	
	ctx := context.Background()
	key := "test-key"
	expectedValue := "test-value"
	callCount := int32(0)
	
	fn := func(ctx context.Context) (interface{}, error) {
		atomic.AddInt32(&callCount, 1)
		return expectedValue, nil
	}
	
	// 第一次调用
	result, err := coalescer.Execute(ctx, key, fn)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	if result != expectedValue {
		t.Errorf("Expected %v, got %v", expectedValue, result)
	}
	
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("Expected function to be called once, called %d times", callCount)
	}
}

func TestQueryCoalescerConcurrent(t *testing.T) {
	config := &CoalescingConfig{
		Timeout:      5 * time.Second,
		CleanupDelay: 100 * time.Millisecond,
	}
	coalescer := NewQueryCoalescer(config)
	defer coalescer.Close()
	
	ctx := context.Background()
	key := "test-key"
	expectedValue := "test-value"
	callCount := int32(0)
	
	fn := func(ctx context.Context) (interface{}, error) {
		atomic.AddInt32(&callCount, 1)
		time.Sleep(100 * time.Millisecond) // 模拟耗时操作
		return expectedValue, nil
	}
	
	// 并发执行多个相同查询
	goroutines := 10
	results := make(chan interface{}, goroutines)
	errors := make(chan error, goroutines)
	
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := coalescer.Execute(ctx, key, fn)
			if err != nil {
				errors <- err
			} else {
				results <- result
			}
		}()
	}
	
	wg.Wait()
	close(results)
	close(errors)
	
	// 检查错误
	for err := range errors {
		t.Errorf("Unexpected error: %v", err)
	}
	
	// 检查结果
	resultCount := 0
	for result := range results {
		if result != expectedValue {
			t.Errorf("Expected %v, got %v", expectedValue, result)
		}
		resultCount++
	}
	
	if resultCount != goroutines {
		t.Errorf("Expected %d results, got %d", goroutines, resultCount)
	}
	
	// 函数应该只被调用一次（查询合并）
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("Expected function to be called once, called %d times", callCount)
	}
	
	// 验证统计信息
	stats := coalescer.GetStats()
	totalRequests := stats["total_requests"].(int64)
	mergedRequests := stats["merged_requests"].(int64)
	savedQueries := stats["saved_queries"].(int64)
	
	if totalRequests != int64(goroutines) {
		t.Errorf("Expected %d total requests, got %d", goroutines, totalRequests)
	}
	
	if mergedRequests != int64(goroutines-1) {
		t.Errorf("Expected %d merged requests, got %d", goroutines-1, mergedRequests)
	}
	
	if savedQueries != int64(goroutines-1) {
		t.Errorf("Expected %d saved queries, got %d", goroutines-1, savedQueries)
	}
}

func TestQueryCoalescerDifferentKeys(t *testing.T) {
	config := &CoalescingConfig{
		Timeout:      5 * time.Second,
		CleanupDelay: 100 * time.Millisecond,
	}
	coalescer := NewQueryCoalescer(config)
	defer coalescer.Close()
	
	ctx := context.Background()
	callCount := int32(0)
	
	fn := func(ctx context.Context) (interface{}, error) {
		atomic.AddInt32(&callCount, 1)
		return "value", nil
	}
	
	// 不同键的查询应该分别执行
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "test-key-" + string(rune('0'+i))
			_, err := coalescer.Execute(ctx, key, fn)
			if err != nil {
				t.Errorf("Unexpected error for key %s: %v", key, err)
			}
		}(i)
	}
	
	wg.Wait()
	
	// 不同键应该各自执行一次
	if atomic.LoadInt32(&callCount) != 5 {
		t.Errorf("Expected function to be called 5 times, called %d times", callCount)
	}
}

func TestQueryCoalescerError(t *testing.T) {
	config := &CoalescingConfig{
		Timeout:      5 * time.Second,
		CleanupDelay: 100 * time.Millisecond,
	}
	coalescer := NewQueryCoalescer(config)
	defer coalescer.Close()
	
	ctx := context.Background()
	key := "test-key"
	expectedError := errors.New("test error")
	
	fn := func(ctx context.Context) (interface{}, error) {
		return nil, expectedError
	}
	
	// 并发执行，都应该收到相同的错误
	goroutines := 5
	var wg sync.WaitGroup
	errorChan := make(chan error, goroutines)
	
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := coalescer.Execute(ctx, key, fn)
			errorChan <- err
		}()
	}
	
	wg.Wait()
	close(errorChan)
	
	// 检查所有协程都收到了错误
	errorCount := 0
	for err := range errorChan {
		if err == nil {
			t.Error("Expected error, got nil")
		} else if err.Error() != expectedError.Error() {
			t.Errorf("Expected error '%v', got '%v'", expectedError, err)
		}
		errorCount++
	}
	
	if errorCount != goroutines {
		t.Errorf("Expected %d errors, got %d", goroutines, errorCount)
	}
}

func TestQueryCoalescerTimeout(t *testing.T) {
	config := &CoalescingConfig{
		Timeout:      100 * time.Millisecond, // 短超时时间
		CleanupDelay: 50 * time.Millisecond,
	}
	coalescer := NewQueryCoalescer(config)
	defer coalescer.Close()
	
	ctx := context.Background()
	key := "test-key"
	
	fn := func(ctx context.Context) (interface{}, error) {
		time.Sleep(200 * time.Millisecond) // 超过组超时时间
		return "value", nil
	}
	
	// 第一个查询启动
	done1 := make(chan struct{})
	go func() {
		_, err := coalescer.Execute(ctx, key, fn)
		if err != nil {
			t.Logf("First query error (expected): %v", err)
		}
		close(done1)
	}()
	
	// 等待一段时间让第一个查询开始
	time.Sleep(150 * time.Millisecond)
	
	// 第二个查询应该创建新的组（第一个已超时）
	result, err := coalescer.Execute(ctx, key, fn)
	if err != nil {
		t.Logf("Second query error: %v", err)
	} else if result != "value" {
		t.Errorf("Expected 'value', got %v", result)
	}
	
	<-done1 // 等待第一个查询完成
}

func TestQueryCoalescerContext(t *testing.T) {
	config := &CoalescingConfig{
		Timeout:      5 * time.Second,
		CleanupDelay: 100 * time.Millisecond,
	}
	coalescer := NewQueryCoalescer(config)
	defer coalescer.Close()
	
	ctx, cancel := context.WithCancel(context.Background())
	key := "test-key"
	
	fn := func(ctx context.Context) (interface{}, error) {
		time.Sleep(200 * time.Millisecond)
		return "value", nil
	}
	
	// 启动查询并立即取消上下文
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	
	_, err := coalescer.Execute(ctx, key, fn)
	if err == nil {
		t.Error("Expected context cancellation error")
	}
	
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestQueryCoalescerStats(t *testing.T) {
	config := &CoalescingConfig{
		Timeout:      5 * time.Second,
		CleanupDelay: 100 * time.Millisecond,
	}
	coalescer := NewQueryCoalescer(config)
	defer coalescer.Close()
	
	// 重置统计
	coalescer.stats.Reset()
	
	ctx := context.Background()
	fn := func(ctx context.Context) (interface{}, error) {
		time.Sleep(10 * time.Millisecond)
		return "value", nil
	}
	
	// 执行一些查询
	for i := 0; i < 3; i++ {
		key := "test-key"
		var wg sync.WaitGroup
		
		// 每个键执行多个并发查询
		for j := 0; j < 5; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := coalescer.Execute(ctx, key, fn)
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}()
		}
		wg.Wait()
		
		// 等待一点时间确保组被清理
		time.Sleep(50 * time.Millisecond)
		key = key + string(rune('0'+i)) // 更改键以避免合并
	}
	
	stats := coalescer.GetStats()
	
	// 验证统计信息
	if stats["total_requests"].(int64) == 0 {
		t.Error("Expected non-zero total requests")
	}
	
	if stats["merged_requests"].(int64) == 0 {
		t.Error("Expected non-zero merged requests")
	}
	
	if stats["saved_queries"].(int64) == 0 {
		t.Error("Expected non-zero saved queries")
	}
	
	savingsRate := coalescer.stats.GetSavingsRate()
	if savingsRate <= 0 {
		t.Errorf("Expected positive savings rate, got %f", savingsRate)
	}
}

func TestQueryCoalescerClose(t *testing.T) {
	config := &CoalescingConfig{
		Timeout:      5 * time.Second,
		CleanupDelay: 100 * time.Millisecond,
	}
	coalescer := NewQueryCoalescer(config)
	
	ctx := context.Background()
	key := "test-key"
	
	// 启动一些查询
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn := func(ctx context.Context) (interface{}, error) {
				time.Sleep(100 * time.Millisecond)
				return "value", nil
			}
			
			_, err := coalescer.Execute(ctx, key, fn)
			if err != nil && err.Error() != "query coalescer is closing" {
				t.Errorf("Unexpected error: %v", err)
			}
		}()
	}
	
	// 等待一点时间让查询开始
	time.Sleep(20 * time.Millisecond)
	
	// 关闭合并器
	err := coalescer.Close()
	if err != nil {
		t.Errorf("Unexpected close error: %v", err)
	}
	
	wg.Wait()
	
	// 关闭后的查询应该失败
	fn := func(ctx context.Context) (interface{}, error) {
		return "value", nil
	}
	
	_, err = coalescer.Execute(ctx, "new-key", fn)
	if err == nil {
		t.Error("Expected error after close")
	}
}

func TestQueryCoalescerActiveGroups(t *testing.T) {
	config := &CoalescingConfig{
		Timeout:      5 * time.Second,
		CleanupDelay: 100 * time.Millisecond,
	}
	coalescer := NewQueryCoalescer(config)
	defer coalescer.Close()
	
	ctx := context.Background()
	
	// 初始状态应该没有活跃组
	if coalescer.ActiveGroupsCount() != 0 {
		t.Errorf("Expected 0 active groups, got %d", coalescer.ActiveGroupsCount())
	}
	
	if coalescer.HasActiveGroup("nonexistent") {
		t.Error("Should not have active group for nonexistent key")
	}
	
	// 启动一个长时间运行的查询
	key := "test-key"
	done := make(chan struct{})
	
	go func() {
		defer close(done)
		fn := func(ctx context.Context) (interface{}, error) {
			time.Sleep(200 * time.Millisecond)
			return "value", nil
		}
		
		_, err := coalescer.Execute(ctx, key, fn)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	}()
	
	// 等待查询开始
	time.Sleep(50 * time.Millisecond)
	
	// 现在应该有一个活跃组
	if coalescer.ActiveGroupsCount() != 1 {
		t.Errorf("Expected 1 active group, got %d", coalescer.ActiveGroupsCount())
	}
	
	if !coalescer.HasActiveGroup(key) {
		t.Error("Should have active group for test key")
	}
	
	// 等待查询完成
	<-done
	
	// 等待组被清理
	time.Sleep(100 * time.Millisecond)
	
	// 活跃组数应该回到0
	if coalescer.ActiveGroupsCount() != 0 {
		t.Errorf("Expected 0 active groups after completion, got %d", coalescer.ActiveGroupsCount())
	}
}

func TestCoalescingStatsReset(t *testing.T) {
	stats := &CoalescingStats{}
	
	// 设置一些值
	stats.TotalRequests = 100
	stats.MergedRequests = 50
	stats.SavedQueries = 50
	
	// 重置
	stats.Reset()
	
	// 验证所有值都被重置
	if stats.TotalRequests != 0 {
		t.Errorf("Expected TotalRequests to be 0, got %d", stats.TotalRequests)
	}
	
	if stats.MergedRequests != 0 {
		t.Errorf("Expected MergedRequests to be 0, got %d", stats.MergedRequests)
	}
	
	if stats.SavedQueries != 0 {
		t.Errorf("Expected SavedQueries to be 0, got %d", stats.SavedQueries)
	}
	
	if stats.GetSavingsRate() != 0.0 {
		t.Errorf("Expected SavingsRate to be 0.0, got %f", stats.GetSavingsRate())
	}
}