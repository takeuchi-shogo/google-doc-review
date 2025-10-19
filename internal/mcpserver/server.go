package mcpserver

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/takeuchi-shogo/google-doc-review/config"
	"github.com/takeuchi-shogo/google-doc-review/internal/authmanager"
	"github.com/takeuchi-shogo/google-doc-review/internal/review"
)

func Run() error {
	ctx := context.Background()

	// 設定を読み込む
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// MCP serverを起動
	s := server.NewMCPServer(
		"google-doc-review",
		"0.0.1",
		server.WithToolCapabilities(true), // ツール機能を有効化
	)

	// 認証してHTTPクライアントを取得
	authMgr := authmanager.NewWithConfig(
		cfg.Google.ClientID,
		cfg.Google.ClientSecret,
		&authmanager.BrowserAuthenticator{},
	)
	client, err := authMgr.GetOrAuthenticateClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to get authenticated client: %w", err)
	}

	// GoogleDocFetcherを作成
	fetcher := review.NewGoogleDocFetcher(client)

	// ツールを登録
	tool := mcp.NewTool("fetch_google_doc",
		mcp.WithDescription("Fetch content from a Google Doc by URL"),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("The Google Docs URL to fetch"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// URLパラメータを取得
		url, err := request.RequireString("url")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// ドキュメントを取得
		doc, err := fetcher.FetchDocument(ctx, url)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to fetch document: %v", err)), nil
		}

		// 結果を返す
		result := fmt.Sprintf("Title: %s\n\nContent:\n%s", doc.Title, doc.Content)
		return mcp.NewToolResultText(result), nil
	})

	// Start the stdio server
	if err := server.ServeStdio(s); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}
