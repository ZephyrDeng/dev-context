package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	
	"frontend-news-mcp/internal/cache"
	"frontend-news-mcp/internal/collector"
	"frontend-news-mcp/internal/formatter"
	"frontend-news-mcp/internal/processor"
)

// Handler MCP工具处理器，负责注册和处理所有MCP工具调用
type Handler struct {
	weeklyNewsService    *WeeklyNewsService
	topicSearchService   *TopicSearchService
	trendingReposService *TrendingReposService
	validator            *Validator
}

// NewHandler 创建新的MCP工具处理器
func NewHandler(
	cacheManager *cache.CacheManager,
	collectorMgr *collector.CollectorManager,
	processor *processor.Processor,
	formatterFactory *formatter.FormatterFactory,
) *Handler {
	return &Handler{
		weeklyNewsService:    NewWeeklyNewsService(cacheManager, collectorMgr, processor, formatterFactory),
		topicSearchService:   NewTopicSearchService(cacheManager, collectorMgr, processor, formatterFactory),
		trendingReposService: NewTrendingReposService(cacheManager, collectorMgr, processor, formatterFactory),
		validator:            NewValidator(),
	}
}

// RegisterTools 注册所有MCP工具到服务器
func (h *Handler) RegisterTools(server *mcp.Server) error {
	// 注册周报新闻工具
	if err := h.registerWeeklyNewsTools(server); err != nil {
		return fmt.Errorf("注册周报新闻工具失败: %w", err)
	}
	
	// 注册主题搜索工具
	if err := h.registerTopicSearchTools(server); err != nil {
		return fmt.Errorf("注册主题搜索工具失败: %w", err)
	}
	
	// 注册热门仓库工具
	if err := h.registerTrendingReposTools(server); err != nil {
		return fmt.Errorf("注册热门仓库工具失败: %w", err)
	}
	
	log.Printf("成功注册 %d 个MCP工具", 3)
	return nil
}

// registerWeeklyNewsTools 注册周报新闻相关工具
func (h *Handler) registerWeeklyNewsTools(server *mcp.Server) error {
	// TODO: 临时简化实现，后续完善工具注册
	log.Printf("周报新闻工具注册已预留")
	return nil
}

// registerTopicSearchTools 注册主题搜索相关工具
func (h *Handler) registerTopicSearchTools(server *mcp.Server) error {
	// TODO: 临时简化实现，后续完善工具注册
	log.Printf("主题搜索工具注册已预留")
	return nil
}

// registerTrendingReposTools 注册热门仓库相关工具
func (h *Handler) registerTrendingReposTools(server *mcp.Server) error {
	// TODO: 临时简化实现，后续完善工具注册
	log.Printf("热门仓库工具注册已预留")
	return nil
}

// handleGetWeeklyFrontendNews 处理周报新闻工具调用
func (h *Handler) handleGetWeeklyFrontendNews(
	ctx context.Context,
	req *mcp.CallToolRequest,
	args WeeklyNewsParams,
) (*mcp.CallToolResult, any, error) {
	log.Printf("处理 get_weekly_frontend_news 请求: category=%s, timeRange=%s-%s", 
		args.Category, args.StartDate, args.EndDate)
	
	// 参数验证
	if err := h.validator.ValidateWeeklyNewsParams(args); err != nil {
		return nil, nil, fmt.Errorf("参数验证失败: %w", err)
	}
	
	// 调用服务
	result, err := h.weeklyNewsService.GetWeeklyFrontendNews(ctx, args)
	if err != nil {
		return nil, nil, fmt.Errorf("获取周报新闻失败: %w", err)
	}
	
	// 格式化输出
	format := args.Format
	if format == "" {
		format = "json"
	}
	
	var content string
	if format == "json" {
		// JSON格式直接返回结构化数据
		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return nil, nil, fmt.Errorf("JSON序列化失败: %w", err)
		}
		content = string(jsonData)
	} else {
		// 其他格式使用formatter
		content, err = h.weeklyNewsService.FormatResult(result, format)
		if err != nil {
			return nil, nil, fmt.Errorf("格式化结果失败: %w", err)
		}
	}
	
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, result, nil
}

// handleSearchFrontendTopic 处理主题搜索工具调用
func (h *Handler) handleSearchFrontendTopic(
	ctx context.Context,
	req *mcp.CallToolRequest,
	args TopicSearchParams,
) (*mcp.CallToolResult, any, error) {
	log.Printf("处理 search_frontend_topic 请求: query=%s, platform=%s", 
		args.Query, args.Platform)
	
	// 参数验证
	if err := h.validator.ValidateTopicSearchParams(args); err != nil {
		return nil, nil, fmt.Errorf("参数验证失败: %w", err)
	}
	
	// 调用服务
	result, err := h.topicSearchService.SearchFrontendTopic(ctx, args)
	if err != nil {
		return nil, nil, fmt.Errorf("搜索前端主题失败: %w", err)
	}
	
	// 格式化输出
	format := args.Format
	if format == "" {
		format = "json"
	}
	
	var content string
	if format == "json" {
		// JSON格式直接返回结构化数据
		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return nil, nil, fmt.Errorf("JSON序列化失败: %w", err)
		}
		content = string(jsonData)
	} else {
		// 其他格式使用formatter
		content, err = h.topicSearchService.FormatResult(result, format)
		if err != nil {
			return nil, nil, fmt.Errorf("格式化结果失败: %w", err)
		}
	}
	
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, result, nil
}

// handleGetTrendingRepositories 处理热门仓库工具调用
func (h *Handler) handleGetTrendingRepositories(
	ctx context.Context,
	req *mcp.CallToolRequest,
	args TrendingReposParams,
) (*mcp.CallToolResult, any, error) {
	log.Printf("处理 get_trending_repositories 请求: language=%s, timeRange=%s", 
		args.Language, args.TimeRange)
	
	// 参数验证
	if err := h.validator.ValidateTrendingReposParams(args); err != nil {
		return nil, nil, fmt.Errorf("参数验证失败: %w", err)
	}
	
	// 调用服务
	result, err := h.trendingReposService.GetTrendingRepositories(ctx, args)
	if err != nil {
		return nil, nil, fmt.Errorf("获取热门仓库失败: %w", err)
	}
	
	// 格式化输出
	format := args.Format
	if format == "" {
		format = "json"
	}
	
	var content string
	if format == "json" {
		// JSON格式直接返回结构化数据
		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return nil, nil, fmt.Errorf("JSON序列化失败: %w", err)
		}
		content = string(jsonData)
	} else {
		// 其他格式使用formatter
		content, err = h.trendingReposService.FormatResult(result, format)
		if err != nil {
			return nil, nil, fmt.Errorf("格式化结果失败: %w", err)
		}
	}
	
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, result, nil
}

// GetToolsInfo 获取工具信息列表
func (h *Handler) GetToolsInfo() []ToolInfo {
	return []ToolInfo{
		{
			Name:        "get_weekly_frontend_news",
			Description: "获取指定时间范围内的前端开发资讯和新闻",
			Category:    "News",
			Parameters:  []string{"startDate", "endDate", "category", "minQuality", "maxResults", "format"},
			Examples: []string{
				"获取最近7天的React相关新闻",
				"获取本月的高质量前端文章",
				"获取指定时间范围的TypeScript资讯",
			},
		},
		{
			Name:        "search_frontend_topic",
			Description: "基于关键词搜索相关前端主题和讨论",
			Category:    "Search",
			Parameters:  []string{"query", "language", "platform", "timeRange", "maxResults", "format"},
			Examples: []string{
				"搜索React Hooks相关讨论",
				"查找Vue 3性能优化话题",
				"搜索TypeScript最佳实践",
			},
		},
		{
			Name:        "get_trending_repositories",
			Description: "获取GitHub上的热门前端相关仓库",
			Category:    "Repositories",
			Parameters:  []string{"language", "timeRange", "minStars", "maxResults", "category", "format"},
			Examples: []string{
				"获取本周热门JavaScript仓库",
				"查找最新的React组件库",
				"获取高星标的前端工具项目",
			},
		},
	}
}

// ToolInfo 工具信息
type ToolInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Parameters  []string `json:"parameters"`
	Examples    []string `json:"examples"`
}