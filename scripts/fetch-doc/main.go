package main

import (
	"context"
	"fmt"
	"log"

	"github.com/takeuchi-shogo/google-doc-review/config"
	"github.com/takeuchi-shogo/google-doc-review/internal/authmanager"
	"github.com/takeuchi-shogo/google-doc-review/internal/review"
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

	// GoogleDocFetcherを作成
	fetcher := review.NewGoogleDocFetcher(client)

	// ドキュメントIDを指定
	docURL := "https://docs.google.com/document/d/1UKUfFhraETmAQIG-sQun_Ctga0UE6jOq9zfpDmarErQ/edit?tab=t.0#heading=h.62f0wfno6lbr"

	// ドキュメントを取得
	doc, err := fetcher.FetchDocument(ctx, docURL)
	if err != nil {
		log.Fatalf("failed to fetch document: %v", err)
	}

	// 結果を表示
	fmt.Printf("Title: %s\n\n", doc.Title)
	fmt.Printf("Content:\n%s\n", doc.Content)
}
