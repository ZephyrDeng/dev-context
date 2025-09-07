// Package main provides the entry point for the frontend-news-mcp server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"frontend-news-mcp/internal/cache"
	"frontend-news-mcp/internal/collector"
	"frontend-news-mcp/internal/formatter"
	"frontend-news-mcp/internal/mcp"
	"frontend-news-mcp/internal/processor"
	"frontend-news-mcp/internal/tools"
)

var (
	version = "v0.1.0" // This will be set by build process
	commit  = "dev"    // This will be set by build process
)

func main() {
	// Define command line flags
	var (
		configFile = flag.String("config", "", "Path to configuration file")
		logLevel   = flag.String("log-level", "info", "Logging level (debug, info, warn, error)")
		showVer    = flag.Bool("version", false, "Show version information")
		transport  = flag.String("transport", "stdio", "Transport type (stdio, http, websocket)")
		addr       = flag.String("addr", ":8080", "Address to bind (for http/websocket transports)")
	)
	flag.Parse()

	// Show version and exit
	if *showVer {
		fmt.Printf("frontend-news-mcp %s (commit: %s)\n", version, commit)
		os.Exit(0)
	}

	// Parse log level
	var level slog.Level
	switch *logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		log.Fatalf("Invalid log level: %s", *logLevel)
	}

	// Create server configuration
	config := &mcp.Config{
		Name:        "frontend-news-mcp",
		Version:     version,
		Description: "Real-time frontend news MCP server",
		LogLevel:    level,
	}

	// Load configuration from file if specified
	if *configFile != "" {
		log.Printf("Loading configuration from: %s", *configFile)
		// TODO: Implement config file loading when needed
	}

	// Create MCP server
	server := mcp.NewServer(config)
	
	// Add basic capabilities
	if err := server.AddBasicCapabilities(); err != nil {
		log.Fatalf("Failed to add basic capabilities: %v", err)
	}

	// Initialize core components  
	cacheManager := initializeCacheManager()
	collectorManager := initializeCollectorManager()
	processor := initializeProcessor()
	formatterFactory := initializeFormatterFactory()

	// Create tools manager
	toolsManager := tools.NewToolsManager(
		cacheManager,
		collectorManager,
		processor,
		formatterFactory,
		10, // max concurrency
	)

	// Create context that cancels on interrupt signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register tools to MCP server
	handler := toolsManager.GetHandler()
	if err := handler.RegisterTools(server.GetServer()); err != nil {
		log.Fatalf("Failed to register MCP tools: %v", err)
	}

	// Warmup cache in background
	go func() {
		if err := toolsManager.WarmupCache(ctx); err != nil {
			log.Printf("Cache warmup failed: %v", err)
		} else {
			log.Printf("Cache warmup completed successfully")
		}
	}()

	log.Printf("MCP Server initialized with %d tools", len(handler.GetToolsInfo()))

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	// Start the server based on transport type
	var err error
	switch *transport {
	case "stdio":
		log.Printf("Starting MCP server on stdio transport")
		err = server.RunStdio(ctx)
	case "http":
		log.Printf("Starting MCP server on HTTP transport at %s", *addr)
		// TODO: Implement HTTP transport when needed
		log.Fatalf("HTTP transport not yet implemented")
	case "websocket":
		log.Printf("Starting MCP server on WebSocket transport at %s", *addr)
		// TODO: Implement WebSocket transport when needed
		log.Fatalf("WebSocket transport not yet implemented")
	default:
		log.Fatalf("Unsupported transport type: %s", *transport)
	}

	// Handle server errors
	if err != nil {
		if ctx.Err() == context.Canceled {
			log.Printf("Server shutdown completed")
		} else {
			log.Fatalf("Server failed: %v", err)
		}
	}

	// Graceful cleanup
	if err := server.Close(); err != nil {
		log.Printf("Error during server cleanup: %v", err)
	}

	log.Printf("Server stopped")
}

func initializeCacheManager() *cache.CacheManager {
	// TODO: 根据实际cache包API调整
	log.Printf("初始化缓存管理器")
	// 暂时返回nil，后续根据实际API实现
	return nil
}

func initializeCollectorManager() *collector.CollectorManager {
	// TODO: 根据实际collector包API调整  
	log.Printf("初始化数据采集管理器")
	// 暂时返回nil，后续根据实际API实现
	return nil
}

func initializeProcessor() *processor.Processor {
	return processor.NewProcessor(&processor.Config{
		EnableSummarization:  true,
		EnableSorting:       true,
		MaxSummaryLength:    200,
	})
}

func initializeFormatterFactory() *formatter.FormatterFactory {
	config := formatter.DefaultConfig()
	return formatter.NewFormatterFactory(config)
}