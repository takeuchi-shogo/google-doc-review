package main

import (
	"log"

	"github.com/takeuchi-shogo/google-doc-review/internal/mcpserver"
)

func main() {
	if err := mcpserver.Run(); err != nil {
		log.Fatalf("Failed to run MCP server: %v", err)
	}
}
