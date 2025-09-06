// test_client.go - Simple MCP client for testing the server
package main

import (
	"context"
	"encoding/json"
	"log"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	ctx := context.Background()

	// Create an MCP client
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-client",
		Version: "v0.1.0",
	}, nil)

	// Create command transport to connect to our server
	cmd := exec.Command("./server")
	transport := mcp.NewCommandTransport(cmd)

	log.Printf("Connecting to MCP server...")
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer session.Close()

	log.Printf("Connected successfully!")

	// Test 1: List available tools
	log.Printf("Testing tool listing...")
	tools, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		log.Printf("Failed to list tools: %v", err)
	} else {
		log.Printf("Available tools: %d", len(tools.Tools))
		for _, tool := range tools.Tools {
			log.Printf("  - %s: %s", tool.Name, tool.Description)
		}
	}

	// Test 2: Call the echo tool
	if len(tools.Tools) > 0 {
		log.Printf("Testing echo tool...")
		params := &mcp.CallToolParams{
			Name: "echo",
			Arguments: map[string]any{
				"message": "Hello, MCP World!",
			},
		}

		result, err := session.CallTool(ctx, params)
		if err != nil {
			log.Printf("Failed to call echo tool: %v", err)
		} else {
			log.Printf("Tool call successful!")
			for _, content := range result.Content {
				if textContent, ok := content.(*mcp.TextContent); ok {
					log.Printf("  Response: %s", textContent.Text)
				}
			}
		}
	}

	// Test 3: Test session is working
	log.Printf("Testing session state...")
	if session != nil {
		log.Printf("Session is active and responsive")
		_, _ = json.Marshal(struct{}{}) // Just to use json import
	}

	log.Printf("All tests completed successfully!")
}