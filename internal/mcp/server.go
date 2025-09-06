// Package mcp provides the core MCP server implementation.
package mcp

import (
	"context"
	"log"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Config represents the configuration for the MCP server
type Config struct {
	Name        string
	Version     string
	Description string
	LogLevel    slog.Level
}

// DefaultConfig returns a default configuration for the MCP server
func DefaultConfig() *Config {
	return &Config{
		Name:        "frontend-news-mcp",
		Version:     "v0.1.0",
		Description: "Real-time frontend news MCP server",
		LogLevel:    slog.LevelInfo,
	}
}

// Server wraps the MCP SDK server with additional functionality
type Server struct {
	config *Config
	server *mcp.Server
}

// NewServer creates a new MCP server instance with the given configuration
func NewServer(config *Config) *Server {
	if config == nil {
		config = DefaultConfig()
	}

	// Create the MCP implementation
	impl := &mcp.Implementation{
		Name:    config.Name,
		Version: config.Version,
	}

	// Configure server options
	opts := &mcp.ServerOptions{
		// ServerOptions structure will be determined from the actual SDK
	}

	// Create the MCP server
	mcpServer := mcp.NewServer(impl, opts)

	return &Server{
		config: config,
		server: mcpServer,
	}
}

// GetServer returns the underlying MCP server instance
func (s *Server) GetServer() *mcp.Server {
	return s.server
}

// GetConfig returns the server configuration
func (s *Server) GetConfig() *Config {
	return s.config
}

// Run starts the server with the specified transport
func (s *Server) Run(ctx context.Context, transport mcp.Transport) error {
	log.Printf("Starting MCP server %s %s", s.config.Name, s.config.Version)
	return s.server.Run(ctx, transport)
}

// RunStdio starts the server with stdio transport (most common for MCP)
func (s *Server) RunStdio(ctx context.Context) error {
	transport := &mcp.StdioTransport{}
	return s.Run(ctx, transport)
}

// Close gracefully shuts down the server
func (s *Server) Close() error {
	// The MCP SDK server doesn't expose a Close method yet, 
	// but we prepare for graceful shutdown here
	log.Printf("Shutting down MCP server %s", s.config.Name)
	return nil
}

// AddBasicCapabilities adds basic server capabilities and tools
func (s *Server) AddBasicCapabilities() error {
	// Add a simple echo tool for testing
	type EchoArgs struct {
		Message string `json:"message" jsonschema:"the message to echo"`
	}

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "echo",
		Description: "Echo a message back to test MCP functionality",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args EchoArgs) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Echo: " + args.Message},
			},
		}, nil, nil
	})

	log.Printf("Basic server capabilities initialized for %s", s.config.Name)
	return nil
}