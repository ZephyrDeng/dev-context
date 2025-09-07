package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	
	"frontend-news-mcp/internal/cache"
	"frontend-news-mcp/internal/collector"
	"frontend-news-mcp/internal/formatter"
	"frontend-news-mcp/internal/mcp"
	"frontend-news-mcp/internal/processor"
	"frontend-news-mcp/internal/tools"
)

// 使用示例：如何集成和使用核心MCP工具
func main() {
	ctx := context.Background()

	// 1. 初始化核心组件
	cacheManager := initializeCacheManager()
	collectorManager := initializeCollectorManager()
	processor := initializeProcessor()
	formatterFactory := initializeFormatterFactory()

	// 2. 创建工具管理器
	toolsManager := tools.NewToolsManager(
		cacheManager,
		collectorManager,
		processor,
		formatterFactory,
		10, // 最大并发数
	)

	// 3. 预热缓存
	if err := toolsManager.WarmupCache(ctx); err != nil {
		log.Printf("缓存预热失败: %v", err)
	}

	// 4. 创建MCP服务器
	mcpConfig := mcp.DefaultConfig()
	mcpServer := mcp.NewServer(mcpConfig)

	// 5. 注册工具到MCP服务器
	handler := toolsManager.GetHandler()
	if err := handler.RegisterTools(mcpServer.GetServer()); err != nil {
		log.Fatalf("注册MCP工具失败: %v", err)
	}

	// 6. 启动健康检查
	go healthCheckLoop(ctx, toolsManager)

	// 7. 演示工具使用
	demoToolUsage(ctx, handler)

	// 8. 启动MCP服务器
	log.Printf("启动MCP服务器...")
	if err := mcpServer.RunStdio(ctx); err != nil {
		log.Fatalf("MCP服务器启动失败: %v", err)
	}
}

func initializeCacheManager() *cache.Manager {
	config := &cache.Config{
		MaxSize:     10000,
		DefaultTTL:  time.Hour,
		CleanupInterval: 10 * time.Minute,
	}
	return cache.NewManager(config)
}

func initializeCollectorManager() *collector.Manager {
	return collector.NewManager(&collector.Config{
		Timeout: 30 * time.Second,
		MaxRetries: 3,
		UserAgent: "FrontendNews-MCP/1.0",
	})
}

func initializeProcessor() *processor.Processor {
	return processor.NewProcessor(&processor.Config{
		EnableSummarization: true,
		EnableSorting: true,
		MaxSummaryLength: 200,
	})
}

func initializeFormatterFactory() *formatter.FormatterFactory {
	config := formatter.DefaultConfig()
	return formatter.NewFormatterFactory(config)
}

func healthCheckLoop(ctx context.Context, toolsManager *tools.ToolsManager) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			health := toolsManager.HealthCheck(ctx)
			stats := toolsManager.GetStats()
			
			log.Printf("健康检查 - 状态: %v, 活跃任务: %d, 缓存命中率: %.2f%%", 
				health["status"], stats.ActiveJobs, stats.CacheHitRate*100)
		}
	}
}

func demoToolUsage(ctx context.Context, handler *tools.Handler) {
	log.Println("=== MCP工具使用演示 ===")

	// 演示1: 获取周报新闻
	log.Println("1. 获取React相关周报新闻")
	weeklyParams := tools.WeeklyNewsParams{
		Category:    "react",
		MinQuality:  0.6,
		MaxResults:  10,
		Format:      "json",
	}

	if result, err := handler.weeklyNewsService.GetWeeklyFrontendNews(ctx, weeklyParams); err != nil {
		log.Printf("获取周报新闻失败: %v", err)
	} else {
		log.Printf("成功获取 %d 篇文章，涵盖 %d 个数据源", 
			len(result.Articles), len(result.Sources))
	}

	// 演示2: 搜索主题
	log.Println("2. 搜索Vue 3相关话题")
	searchParams := tools.TopicSearchParams{
		Query:      "Vue 3 composition api",
		Language:   "javascript",
		MaxResults: 15,
		Format:     "json",
	}

	if result, err := handler.topicSearchService.SearchFrontendTopic(ctx, searchParams); err != nil {
		log.Printf("主题搜索失败: %v", err)
	} else {
		log.Printf("找到 %d 个相关结果：%d 篇文章, %d 个仓库, %d 个讨论", 
			result.TotalResults, len(result.Articles), len(result.Repositories), len(result.Discussions))
	}

	// 演示3: 获取热门仓库
	log.Println("3. 获取本周热门TypeScript仓库")
	reposParams := tools.TrendingReposParams{
		Language:    "typescript",
		TimeRange:   "weekly",
		MinStars:    50,
		MaxResults:  20,
		Format:      "json",
	}

	if result, err := handler.trendingReposService.GetTrendingRepositories(ctx, reposParams); err != nil {
		log.Printf("获取热门仓库失败: %v", err)
	} else {
		log.Printf("找到 %d 个热门仓库，平均星标数: %.0f", 
			len(result.Repositories), result.Summary.AverageStars)
	}

	// 演示4: 获取工具信息
	log.Println("4. 可用工具列表")
	toolsInfo := handler.GetToolsInfo()
	for _, tool := range toolsInfo {
		log.Printf("- %s: %s (分类: %s)", tool.Name, tool.Description, tool.Category)
	}

	log.Println("=== 演示完成 ===")
}

// 错误处理和恢复示例
func handleToolError(err error, toolName string) {
	if validationErrors, ok := err.(tools.ValidationErrors); ok {
		log.Printf("工具 %s 参数验证失败:", toolName)
		for _, validationErr := range validationErrors {
			log.Printf("  - 字段 %s: %s (错误代码: %s)", 
				validationErr.Field, validationErr.Message, validationErr.Code)
		}
	} else {
		log.Printf("工具 %s 执行失败: %v", toolName, err)
	}
}

// 配置示例
type AppConfig struct {
	Cache struct {
		MaxSize         int           `json:"maxSize"`
		DefaultTTL      time.Duration `json:"defaultTTL"`
		CleanupInterval time.Duration `json:"cleanupInterval"`
	} `json:"cache"`
	
	Collector struct {
		Timeout    time.Duration `json:"timeout"`
		MaxRetries int           `json:"maxRetries"`
		UserAgent  string        `json:"userAgent"`
	} `json:"collector"`
	
	Tools struct {
		MaxConcurrency int `json:"maxConcurrency"`
		EnableWarmup   bool `json:"enableWarmup"`
	} `json:"tools"`
	
	MCP struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Description string `json:"description"`
	} `json:"mcp"`
}

func loadConfig() *AppConfig {
	return &AppConfig{
		Cache: struct {
			MaxSize         int           `json:"maxSize"`
			DefaultTTL      time.Duration `json:"defaultTTL"`
			CleanupInterval time.Duration `json:"cleanupInterval"`
		}{
			MaxSize:         10000,
			DefaultTTL:      time.Hour,
			CleanupInterval: 10 * time.Minute,
		},
		Collector: struct {
			Timeout    time.Duration `json:"timeout"`
			MaxRetries int           `json:"maxRetries"`
			UserAgent  string        `json:"userAgent"`
		}{
			Timeout:    30 * time.Second,
			MaxRetries: 3,
			UserAgent:  "FrontendNews-MCP/1.0",
		},
		Tools: struct {
			MaxConcurrency int `json:"maxConcurrency"`
			EnableWarmup   bool `json:"enableWarmup"`
		}{
			MaxConcurrency: 10,
			EnableWarmup:   true,
		},
		MCP: struct {
			Name        string `json:"name"`
			Version     string `json:"version"`
			Description string `json:"description"`
		}{
			Name:        "frontend-news-mcp",
			Version:     "v1.0.0",
			Description: "前端开发新闻和资源MCP服务器",
		},
	}
}