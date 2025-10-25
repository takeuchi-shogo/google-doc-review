package main

import (
	"context"
	"fmt"
	"log"

	"github.com/takeuchi-shogo/google-doc-review/config"
	"github.com/takeuchi-shogo/google-doc-review/internal/authmanager"
	"github.com/takeuchi-shogo/google-doc-review/internal/comment"
)

func main() {
	ctx := context.Background()

	// 設定を読み込む
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// 認証してHTTPクライアントを取得
	authMgr := authmanager.NewWithConfig(
		cfg.Google.ClientID,
		cfg.Google.ClientSecret,
		&authmanager.BrowserAuthenticator{},
	)
	client, err := authMgr.GetOrAuthenticateClient(ctx)
	if err != nil {
		log.Fatalf("failed to get authenticated client: %v", err)
	}

	// CommentManagerを作成
	commentMgr, err := comment.NewCommentManager(client)
	if err != nil {
		log.Fatalf("failed to create comment manager: %v", err)
	}

	// ドキュメントIDを設定から取得
	docID := cfg.Google.TestDocID
	if docID == "" {
		log.Fatal("GOOGLE_TEST_DOC_ID is required. Please set it in .env file or environment variable.")
	}

	// レビューコメントを作成
	issues := []comment.Issue{
		{
			Type:        comment.IssueTypeGrammar,
			Severity:    comment.SeverityCritical,
			LineNumber:  0, // タイトル行
			TextContent: "テストデザインドッグ",
			Suggestion:  "「ドッグ」を「ドキュメント」に修正してください",
			Description: "タイトルに誤字があります。dog(犬)ではなくdocument(文書)が正しいです",
		},
		{
			Type:        comment.IssueTypeMissing,
			Severity:    comment.SeverityCritical,
			LineNumber:  1,
			TextContent: "[Design Doc] テストデザインドッグ",
			Suggestion:  "ドキュメントの目的、背景、対象読者を追加してください",
			Description: "デザインドキュメントとして必須の「概要」セクションが不足しています",
		},
		{
			Type:        comment.IssueTypeStructure,
			Severity:    comment.SeverityWarning,
			LineNumber:  3,
			TextContent: "テストデザインドッグです。",
			Suggestion:  "以下のセクション構成を追加することを推奨します:\n- 背景・目的\n- 設計概要\n- 詳細設計\n- 実装計画\n- テスト計画\n- リスクと対策",
			Description: "デザインドキュメントとして適切なセクション構成が必要です",
		},
		{
			Type:        comment.IssueTypeMissing,
			Severity:    comment.SeverityCritical,
			LineNumber:  5,
			TextContent: "テストテスト",
			Suggestion:  "具体的な設計内容、システムアーキテクチャ、技術選定理由などを記載してください",
			Description: "内容が不十分です。「テストテスト」だけでは設計意図が伝わりません",
		},
	}

	// コメントを作成
	responses, err := commentMgr.CreateCommentsFromIssues(ctx, docID, issues)
	if err != nil {
		log.Printf("警告: 一部のコメント作成に失敗しました: %v", err)
	}

	// 結果を表示
	fmt.Printf("\n✅ %d件のレビューコメントを作成しました:\n\n", len(responses))
	for i, resp := range responses {
		fmt.Printf("%d. Comment ID: %s\n", i+1, resp.CommentID)
		fmt.Printf("   内容: %s\n", resp.Content)
		fmt.Printf("   作成日時: %s\n\n", resp.CreatedAt)
	}
}
