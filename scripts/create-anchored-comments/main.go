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

	// まず既存のコメントを削除
	fmt.Println("既存のコメントを削除中...")
	existingComments, err := commentMgr.ListComments(ctx, docID)
	if err != nil {
		log.Printf("警告: コメント一覧取得に失敗: %v", err)
	} else {
		for _, c := range existingComments {
			if err := commentMgr.DeleteComment(ctx, docID, c.Id); err != nil {
				log.Printf("警告: コメント削除に失敗 (ID: %s): %v", c.Id, err)
			} else {
				fmt.Printf("  削除: %s\n", c.Id)
			}
		}
	}

	fmt.Println("\n新しいアンカー付きレビューコメントを作成中...")
	fmt.Println("注意: CreateAnchoredCommentはLineNumberベースのアンカーを使用しますが、")
	fmt.Println("     Google Docs UIでは正しく表示されない可能性があります。")

	// CreateAnchoredCommentを使ってアンカー付きコメントを作成
	anchoredComments := []struct {
		lineNumber int
		quotedText string
		content    string
		issueType  string
	}{
		{
			lineNumber: 1,
			quotedText: "テストデザインドッグ",
			content:    "🔴 誤字修正: タイトルに誤字があります。「ドッグ(dog)」ではなく「ドキュメント(document)」が正しいです。\n\n提案: 「テストデザインドキュメント」に修正してください。",
			issueType:  "grammar",
		},
		{
			lineNumber: 1,
			quotedText: "[Design Doc] テストデザインドッグ",
			content:    "🔴 概要セクション不足: デザインドキュメントとして必須の「概要」セクションが不足しています。\n\n提案: 以下の内容を追加してください:\n- ドキュメントの目的\n- 背景・課題\n- 対象読者\n- スコープ(範囲)",
			issueType:  "missing",
		},
		{
			lineNumber: 3,
			quotedText: "テストデザインドッグです。",
			content:    "⚠️ 構造改善: デザインドキュメントとして適切なセクション構成が必要です。\n\n推奨セクション:\n1. 概要・目的\n2. 背景・課題\n3. 提案する設計\n4. システムアーキテクチャ\n5. 技術選定と理由\n6. 実装計画\n7. テスト戦略\n8. リスクと対策\n9. 代替案の検討",
			issueType:  "structure",
		},
		{
			lineNumber: 5,
			quotedText: "テストテスト",
			content:    "🔴 内容不足: 設計内容が不十分です。「テストテスト」だけでは設計意図が伝わりません。\n\n提案: 以下の内容を記載してください:\n- 具体的なシステム設計\n- アーキテクチャ図\n- データモデル\n- API仕様\n- 技術スタック選定理由\n- パフォーマンス要件\n- セキュリティ考慮事項",
			issueType:  "missing",
		},
	}

	successCount := 0
	for i, ac := range anchoredComments {
		req := &comment.CommentRequest{
			FileID:     docID,
			Content:    ac.content,
			QuotedText: ac.quotedText,
			LineNumber: ac.lineNumber,
			LineLength: 1,
		}

		// CreateAnchoredCommentを使う(LineNumberベースのアンカー)
		resp, err := commentMgr.CreateAnchoredComment(ctx, req)
		if err != nil {
			log.Printf("❌ コメント%d作成失敗: %v", i+1, err)
			continue
		}

		successCount++
		fmt.Printf("\n✅ コメント%d作成成功:\n", i+1)
		fmt.Printf("   ID: %s\n", resp.CommentID)
		fmt.Printf("   行番号: %d\n", ac.lineNumber)
		fmt.Printf("   引用テキスト: %s\n", ac.quotedText)
		fmt.Printf("   Issue種別: %s\n", ac.issueType)
		fmt.Printf("   アンカー情報: %s\n", resp.Anchor)
		fmt.Printf("   作成日時: %s\n", resp.CreatedAt)
	}

	fmt.Printf("\n\n========================================\n")
	fmt.Printf("✅ 完了: %d/%d件のアンカー付きコメントを作成しました\n", successCount, len(anchoredComments))
	fmt.Printf("========================================\n")
	fmt.Println("\n⚠️ 注意:")
	fmt.Println("LineNumberベースのアンカーはGoogle Docs UIで正しく動作しない可能性があります。")
	fmt.Println("詳細は docs/reports/google-doc/comment-with-anchor.md を参照してください。")
}
