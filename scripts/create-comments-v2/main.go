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

	// ドキュメントID
	docID := "1UKUfFhraETmAQIG-sQun_Ctga0UE6jOq9zfpDmarErQ"

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

	fmt.Println("\n新しいレビューコメントを作成中...")

	// QuotedTextを使ってアンカー付きコメントを作成
	reviewComments := []struct {
		quotedText string
		content    string
		severity   string
	}{
		{
			quotedText: "テストデザインドッグ",
			content:    "🔴 誤字修正: タイトルに誤字があります。「ドッグ(dog)」ではなく「ドキュメント(document)」が正しいです。\n\n提案: 「テストデザインドキュメント」に修正してください。",
			severity:   "critical",
		},
		{
			quotedText: "[Design Doc] テストデザインドッグ",
			content:    "🔴 概要セクション不足: デザインドキュメントとして必須の「概要」セクションが不足しています。\n\n提案: 以下の内容を追加してください:\n- ドキュメントの目的\n- 背景・課題\n- 対象読者\n- スコープ(範囲)",
			severity:   "critical",
		},
		{
			quotedText: "テストデザインドッグです。",
			content:    "⚠️ 構造改善: デザインドキュメントとして適切なセクション構成が必要です。\n\n推奨セクション:\n1. 概要・目的\n2. 背景・課題\n3. 提案する設計\n4. システムアーキテクチャ\n5. 技術選定と理由\n6. 実装計画\n7. テスト戦略\n8. リスクと対策\n9. 代替案の検討",
			severity:   "warning",
		},
		{
			quotedText: "テストテスト",
			content:    "🔴 内容不足: 設計内容が不十分です。「テストテスト」だけでは設計意図が伝わりません。\n\n提案: 以下の内容を記載してください:\n- 具体的なシステム設計\n- アーキテクチャ図\n- データモデル\n- API仕様\n- 技術スタック選定理由\n- パフォーマンス要件\n- セキュリティ考慮事項",
			severity:   "critical",
		},
	}

	successCount := 0
	for i, rc := range reviewComments {
		req := &comment.CommentRequest{
			FileID:     docID,
			Content:    rc.content,
			QuotedText: rc.quotedText,
		}

		// CreateCommentを使う(内部でFindTextPositionを使ってアンカーを作成)
		resp, err := commentMgr.CreateComment(ctx, req)
		if err != nil {
			log.Printf("❌ コメント%d作成失敗: %v", i+1, err)
			continue
		}

		successCount++
		fmt.Printf("\n✅ コメント%d作成成功:\n", i+1)
		fmt.Printf("   ID: %s\n", resp.CommentID)
		fmt.Printf("   引用テキスト: %s\n", rc.quotedText)
		fmt.Printf("   アンカー: %s\n", resp.Anchor)
		fmt.Printf("   作成日時: %s\n", resp.CreatedAt)
	}

	fmt.Printf("\n\n========================================\n")
	fmt.Printf("✅ 完了: %d/%d件のコメントを作成しました\n", successCount, len(reviewComments))
	fmt.Printf("========================================\n")
}
