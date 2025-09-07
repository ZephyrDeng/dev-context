package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ZephyrDeng/dev-context/internal/cache"
	"github.com/ZephyrDeng/dev-context/internal/collector"
	"github.com/ZephyrDeng/dev-context/internal/formatter"
	"github.com/ZephyrDeng/dev-context/internal/processor"
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
	registeredCount := 0

	// 注册周报新闻工具
	if err := h.registerWeeklyNewsTools(server); err != nil {
		return fmt.Errorf("注册周报新闻工具失败: %w", err)
	}
	registeredCount++

	// 注册主题搜索工具
	if err := h.registerTopicSearchTools(server); err != nil {
		return fmt.Errorf("注册主题搜索工具失败: %w", err)
	}
	registeredCount++

	// 注册热门仓库工具
	if err := h.registerTrendingReposTools(server); err != nil {
		return fmt.Errorf("注册热门仓库工具失败: %w", err)
	}
	registeredCount++

	log.Printf("成功注册 %d 个MCP工具", registeredCount)
	return nil
}

// registerWeeklyNewsTools 注册周报新闻相关工具
func (h *Handler) registerWeeklyNewsTools(server *mcp.Server) error {
	// 定义周报新闻工具参数
	type WeeklyNewsArgs struct {
		StartDate      string  `json:"startDate,omitempty" jsonschema:"Start date for news collection (YYYY-MM-DD format, optional)"`
		EndDate        string  `json:"endDate,omitempty" jsonschema:"End date for news collection (YYYY-MM-DD format, optional)"`
		Category       string  `json:"category,omitempty" jsonschema:"Technology category filter (react, vue, angular, etc.)"`
		MinQuality     float64 `json:"minQuality,omitempty" jsonschema:"Minimum quality score 0.0-1.0"`
		MaxResults     int     `json:"maxResults,omitempty" jsonschema:"Maximum results (default 50, max 200)"`
		Format         string  `json:"format,omitempty" jsonschema:"Output format (json, markdown, text)"`
		IncludeContent bool    `json:"includeContent,omitempty" jsonschema:"Include full content (default false)"`
		SortBy         string  `json:"sortBy,omitempty" jsonschema:"Sort by (relevance, quality, date, title)"`
		Sources        string  `json:"sources,omitempty" jsonschema:"Comma-separated list of sources"`
	}

	// 注册周报新闻工具
	mcp.AddTool(server, &mcp.Tool{
		Name:        "weekly_news",
		Description: "Get curated weekly frontend development news from multiple sources",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args WeeklyNewsArgs) (*mcp.CallToolResult, any, error) {
		// 转换参数
		params := WeeklyNewsParams{
			StartDate:      args.StartDate,
			EndDate:        args.EndDate,
			Category:       args.Category,
			MinQuality:     args.MinQuality,
			MaxResults:     args.MaxResults,
			Format:         args.Format,
			IncludeContent: args.IncludeContent,
			SortBy:         args.SortBy,
			Sources:        args.Sources,
		}

		// 调用服务
		result, err := h.weeklyNewsService.GetWeeklyFrontendNews(ctx, params)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Error getting weekly news: %v", err)},
				},
			}, nil, nil
		}

		// 格式化输出
		format := params.Format
		if format == "" {
			format = "json"
		}

		var output string
		switch format {
		case "json":
			jsonData, _ := json.MarshalIndent(result, "", "  ")
			output = string(jsonData)
		case "markdown", "text":
			formatted, err := h.weeklyNewsService.FormatResult(result, format)
			if err != nil {
				output = fmt.Sprintf("Error formatting result: %v", err)
			} else {
				output = formatted
			}
		default:
			jsonData, _ := json.MarshalIndent(result, "", "  ")
			output = string(jsonData)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: output},
			},
		}, nil, nil
	})

	log.Printf("周报新闻工具注册成功")
	return nil
}

// registerTopicSearchTools 注册主题搜索相关工具
func (h *Handler) registerTopicSearchTools(server *mcp.Server) error {
	// 定义主题搜索工具参数
	type TopicSearchArgs struct {
		Query       string  `json:"query" jsonschema:"Technology or topic to search for"`
		Language    string  `json:"language,omitempty" jsonschema:"Programming language filter"`
		Platform    string  `json:"platform,omitempty" jsonschema:"Platform filter (github, stackoverflow, reddit)"`
		SortBy      string  `json:"sortBy,omitempty" jsonschema:"Sort by (relevance, date, popularity, stars)"`
		TimeRange   string  `json:"timeRange,omitempty" jsonschema:"Time range (day, week, month, year, all)"`
		MaxResults  int     `json:"maxResults,omitempty" jsonschema:"Maximum results (default 30, max 100)"`
		Format      string  `json:"format,omitempty" jsonschema:"Output format (json, markdown, text)"`
		IncludeCode bool    `json:"includeCode,omitempty" jsonschema:"Include code snippets"`
		MinScore    float64 `json:"minScore,omitempty" jsonschema:"Minimum relevance score 0.0-1.0"`
		SearchType  string  `json:"searchType,omitempty" jsonschema:"Search type (discussions, repositories, articles, all)"`
	}

	// 注册主题搜索工具
	mcp.AddTool(server, &mcp.Tool{
		Name:        "topic_search",
		Description: "Search and analyze specific frontend technologies and topics",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args TopicSearchArgs) (*mcp.CallToolResult, any, error) {
		// 检查必需参数
		if args.Query == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "Error: query parameter is required"},
				},
			}, nil, nil
		}

		// 转换参数
		params := TopicSearchParams{
			Query:       args.Query,
			Language:    args.Language,
			Platform:    args.Platform,
			SortBy:      args.SortBy,
			TimeRange:   args.TimeRange,
			MaxResults:  args.MaxResults,
			Format:      args.Format,
			IncludeCode: args.IncludeCode,
			MinScore:    args.MinScore,
			SearchType:  args.SearchType,
		}

		// 调用服务
		result, err := h.topicSearchService.SearchFrontendTopic(ctx, params)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Error searching topic: %v", err)},
				},
			}, nil, nil
		}

		// 格式化输出
		format := params.Format
		if format == "" {
			format = "json"
		}

		var output string
		switch format {
		case "json":
			jsonData, _ := json.MarshalIndent(result, "", "  ")
			output = string(jsonData)
		case "markdown", "text":
			formatted, err := h.topicSearchService.FormatResult(result, format)
			if err != nil {
				output = fmt.Sprintf("Error formatting result: %v", err)
			} else {
				output = formatted
			}
		default:
			jsonData, _ := json.MarshalIndent(result, "", "  ")
			output = string(jsonData)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: output},
			},
		}, nil, nil
	})

	log.Printf("主题搜索工具注册成功")
	return nil
}

// registerTrendingReposTools 注册热门仓库相关工具
func (h *Handler) registerTrendingReposTools(server *mcp.Server) error {
	// 定义热门仓库工具参数
	type TrendingReposArgs struct {
		Language           string `json:"language,omitempty" jsonschema:"Programming language filter (javascript, typescript, python, etc.)"`
		TimeRange          string `json:"timeRange,omitempty" jsonschema:"Time range (daily, weekly, monthly)"`
		MinStars           int    `json:"minStars,omitempty" jsonschema:"Minimum star count (default 0)"`
		MaxResults         int    `json:"maxResults,omitempty" jsonschema:"Maximum results (default 30, max 100)"`
		Category           string `json:"category,omitempty" jsonschema:"Repository category (framework, library, tool, example)"`
		IncludeForks       bool   `json:"includeForks,omitempty" jsonschema:"Include fork repositories"`
		SortBy             string `json:"sortBy,omitempty" jsonschema:"Sort by (stars, forks, updated, trending)"`
		Format             string `json:"format,omitempty" jsonschema:"Output format (json, markdown, text)"`
		IncludeDescription bool   `json:"includeDescription,omitempty" jsonschema:"Include detailed descriptions"`
		FrontendOnly       bool   `json:"frontendOnly,omitempty" jsonschema:"Only frontend-related repositories"`
	}

	// 注册热门仓库工具
	mcp.AddTool(server, &mcp.Tool{
		Name:        "trending_repos",
		Description: "Get GitHub trending repositories for frontend technologies",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args TrendingReposArgs) (*mcp.CallToolResult, any, error) {
		// 转换参数
		params := TrendingReposParams{
			Language:           args.Language,
			TimeRange:          args.TimeRange,
			MinStars:           args.MinStars,
			MaxResults:         args.MaxResults,
			Category:           args.Category,
			IncludeForks:       args.IncludeForks,
			SortBy:             args.SortBy,
			Format:             args.Format,
			IncludeDescription: args.IncludeDescription,
			FrontendOnly:       args.FrontendOnly,
		}

		// 调用服务
		result, err := h.trendingReposService.GetTrendingRepositories(ctx, params)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Error getting trending repos: %v", err)},
				},
			}, nil, nil
		}

		// 格式化输出
		format := params.Format
		if format == "" {
			format = "json"
		}

		var output string
		switch format {
		case "json":
			jsonData, _ := json.MarshalIndent(result, "", "  ")
			output = string(jsonData)
		case "markdown", "text":
			formatted, err := h.trendingReposService.FormatResult(result, format)
			if err != nil {
				output = fmt.Sprintf("Error formatting result: %v", err)
			} else {
				output = formatted
			}
		default:
			jsonData, _ := json.MarshalIndent(result, "", "  ")
			output = string(jsonData)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: output},
			},
		}, nil, nil
	})

	log.Printf("热门仓库工具注册成功")
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
