package mcpserver

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"google.golang.org/api/docs/v1"
	"google.golang.org/api/option"

	"github.com/takeuchi-shogo/google-doc-review/internal/authmanager"
)

func Run() error {
	// MCP serverを起動
	s := server.NewMCPServer(
		"google-doc-review",
		"0.0.1",
		server.WithToolCapabilities(false),
	)

	authMgr := authmanager.New()
	if err := authMgr.Authenticate(); err != nil {
		return err
	}

	// Start the stdio server
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
		return err
	}

	return nil
}

func fetchDoc(authMgr *authmanager.AuthManager, args map[string]interface{}) (*mcp.CallToolResult, error) {
	docID := args["doc_id"].(string)

	client, err := authMgr.NewClient()
	if err != nil {
		return nil, err
	}

	docsService, err := docs.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	doc, err := docsService.Documents.Get(docID).Do()
	if err != nil {
		return nil, err
	}

	// ドキュメント内容をマークダウン形式に変換
	content := convertToMarkdown(doc)

	return mcp.NewToolResultStructuredOnly(content), nil
}

func convertToMarkdown(doc *docs.Document) string {
	return fmt.Sprintf("# %s\n\n%s", doc.Title, doc.Body.Content)
}
