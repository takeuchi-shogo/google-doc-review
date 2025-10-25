package mcpserver

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/takeuchi-shogo/google-doc-review/config"
	"github.com/takeuchi-shogo/google-doc-review/internal/authmanager"
	"github.com/takeuchi-shogo/google-doc-review/internal/comment"
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

	// CommentManagerを作成
	commentMgr, err := comment.NewCommentManager(client)
	if err != nil {
		return fmt.Errorf("failed to create comment manager: %w", err)
	}

	// ツールを登録
	// 1. fetch_google_doc - ドキュメント取得
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

	// 2. create_comment - コメント作成
	createCommentTool := mcp.NewTool("create_comment",
		mcp.WithDescription("Create a comment on a Google Doc"),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("The Google Docs URL"),
		),
		mcp.WithString("content",
			mcp.Required(),
			mcp.Description("The comment content"),
		),
		mcp.WithString("quoted_text",
			mcp.Description("Optional: Text to quote in the comment"),
		),
	)

	s.AddTool(createCommentTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// パラメータを取得
		url, err := request.RequireString("url")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		content, err := request.RequireString("content")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		quotedText := request.GetString("quoted_text", "")

		// URLからドキュメントIDを抽出
		docID, err := review.ExtractDocumentID(url)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid URL: %v", err)), nil
		}

		// コメントを作成
		req := &comment.CommentRequest{
			FileID:     docID,
			Content:    content,
			QuotedText: quotedText,
		}

		resp, err := commentMgr.CreateComment(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to create comment: %v", err)), nil
		}

		// 結果を返す
		result := fmt.Sprintf("Comment created successfully!\nComment ID: %s\nContent: %s\nCreated at: %s",
			resp.CommentID, resp.Content, resp.CreatedAt)
		return mcp.NewToolResultText(result), nil
	})

	// 3. create_anchored_comment - アンカー付きコメント作成
	createAnchoredCommentTool := mcp.NewTool("create_anchored_comment",
		mcp.WithDescription("Create an anchored comment on a specific line in a Google Doc"),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("The Google Docs URL"),
		),
		mcp.WithString("content",
			mcp.Required(),
			mcp.Description("The comment content"),
		),
		mcp.WithNumber("line_number",
			mcp.Required(),
			mcp.Description("The line number to anchor the comment to"),
		),
		mcp.WithString("quoted_text",
			mcp.Description("Optional: Text to quote in the comment"),
		),
		mcp.WithNumber("line_length",
			mcp.Description("Optional: Length of the line selection (default: 1)"),
		),
	)

	s.AddTool(createAnchoredCommentTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// パラメータを取得
		url, err := request.RequireString("url")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		content, err := request.RequireString("content")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		lineNumber := request.GetInt("line_number", 0)
		if lineNumber == 0 {
			return mcp.NewToolResultError("line_number is required and must be greater than 0"), nil
		}

		quotedText := request.GetString("quoted_text", "")
		lineLength := request.GetInt("line_length", 1)

		// URLからドキュメントIDを抽出
		docID, err := review.ExtractDocumentID(url)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid URL: %v", err)), nil
		}

		// アンカー付きコメントを作成
		req := &comment.CommentRequest{
			FileID:     docID,
			Content:    content,
			QuotedText: quotedText,
			LineNumber: lineNumber,
			LineLength: lineLength,
		}

		resp, err := commentMgr.CreateAnchoredComment(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to create anchored comment: %v", err)), nil
		}

		// 結果を返す
		result := fmt.Sprintf("Anchored comment created successfully!\nComment ID: %s\nContent: %s\nLine: %d\nCreated at: %s",
			resp.CommentID, resp.Content, lineNumber, resp.CreatedAt)
		return mcp.NewToolResultText(result), nil
	})

	// Start the stdio server
	if err := server.ServeStdio(s); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}
