package collector

import (
	"context"
	"time"
)

// Article 表示采集到的文章数据
type Article struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Content     string            `json:"content"`
	Summary     string            `json:"summary"`
	Author      string            `json:"author"`
	URL         string            `json:"url"`
	PublishedAt time.Time         `json:"published_at"`
	Tags        []string          `json:"tags"`
	Source      string            `json:"source"`
	SourceType  string            `json:"source_type"`
	Language    string            `json:"language"`
	Metadata    map[string]string `json:"metadata"`
}

// CollectResult 表示采集结果
type CollectResult struct {
	Articles []Article `json:"articles"`
	Source   string    `json:"source"`
	Error    error     `json:"error,omitempty"`
}

// CollectConfig 表示采集配置
type CollectConfig struct {
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers,omitempty"`
	Timeout     time.Duration     `json:"timeout,omitempty"`
	MaxArticles int               `json:"max_articles,omitempty"`
	Language    string            `json:"language,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// DataCollector 定义数据采集器的统一接口
type DataCollector interface {
	// Collect 采集数据，返回文章列表
	Collect(ctx context.Context, config CollectConfig) (CollectResult, error)
	
	// GetSourceType 返回采集器类型
	GetSourceType() string
	
	// Validate 验证配置是否有效
	Validate(config CollectConfig) error
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries int           `json:"max_retries"`
	RetryDelay time.Duration `json:"retry_delay"`
}

// CollectorManager 管理所有采集器并提供并发采集能力
type CollectorManager interface {
	// RegisterCollector 注册采集器
	RegisterCollector(sourceType string, collector DataCollector)
	
	// CollectAll 并发采集多个数据源
	CollectAll(ctx context.Context, configs []CollectConfig) []CollectResult
	
	// CollectWithRetry 带重试机制的采集
	CollectWithRetry(ctx context.Context, config CollectConfig, retryConfig RetryConfig) (CollectResult, error)
	
	// GetCollector 根据类型获取采集器
	GetCollector(sourceType string) (DataCollector, bool)
}