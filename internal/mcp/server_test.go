package mcp

import (
	"testing"
)

func TestNewServer(t *testing.T) {
	config := &Config{
		Name:        "test-server",
		Version:     "v0.1.0",
		Description: "Test MCP server",
	}

	server := NewServer(config)

	if server == nil {
		t.Fatal("NewServer returned nil")
	}

	if server.GetConfig() == nil {
		t.Fatal("Server config is nil")
	}

	if server.GetConfig().Name != config.Name {
		t.Errorf("Expected name %s, got %s", config.Name, server.GetConfig().Name)
	}

	if server.GetConfig().Version != config.Version {
		t.Errorf("Expected version %s, got %s", config.Version, server.GetConfig().Version)
	}
}

func TestNewServerWithNilConfig(t *testing.T) {
	server := NewServer(nil)

	if server == nil {
		t.Fatal("NewServer returned nil")
	}

	defaultConfig := DefaultConfig()
	if server.GetConfig().Name != defaultConfig.Name {
		t.Errorf("Expected default name %s, got %s", defaultConfig.Name, server.GetConfig().Name)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	if config.Name == "" {
		t.Error("Default config name is empty")
	}

	if config.Version == "" {
		t.Error("Default config version is empty")
	}

	if config.Description == "" {
		t.Error("Default config description is empty")
	}
}

func TestAddBasicCapabilities(t *testing.T) {
	server := NewServer(nil)
	
	err := server.AddBasicCapabilities()
	if err != nil {
		t.Fatalf("AddBasicCapabilities failed: %v", err)
	}

	// Test that we can call the basic capabilities without error
	// More comprehensive testing would require setting up a full MCP session
}

func TestServerGetters(t *testing.T) {
	config := &Config{
		Name:        "test-server",
		Version:     "v1.2.3",
		Description: "A test server",
	}

	server := NewServer(config)

	// Test GetServer returns non-nil MCP server
	if server.GetServer() == nil {
		t.Error("GetServer returned nil")
	}

	// Test GetConfig returns the original config
	retrievedConfig := server.GetConfig()
	if retrievedConfig == nil {
		t.Fatal("GetConfig returned nil")
	}

	if retrievedConfig != config {
		t.Error("GetConfig did not return the original config object")
	}
}

func TestServerClose(t *testing.T) {
	server := NewServer(nil)

	err := server.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

// Integration test to verify the server can be created with proper MCP structure
func TestMCPServerCreation(t *testing.T) {
	server := NewServer(nil)
	
	// Add basic capabilities
	err := server.AddBasicCapabilities()
	if err != nil {
		t.Fatalf("Failed to add basic capabilities: %v", err)
	}

	// Verify that the underlying MCP server was created
	mcpServer := server.GetServer()
	if mcpServer == nil {
		t.Fatal("Underlying MCP server is nil")
	}

	// This is a basic smoke test - more comprehensive testing would require
	// setting up transport and running the server in a separate goroutine
}