# Google Doc Review MCP Server

Google DocsのコンテンツをClaude経由で取得・レビューするためのMCPサーバー。

## 概要

このMCPサーバーは、Google Docs APIを使用してドキュメントの内容を取得し、Claude Codeから利用できるツールを提供します。

## 機能

- **fetch_google_doc**: Google DocsのURLからドキュメント内容を取得

## セットアップ

### 1. Google Cloud Console での設定

1. [Google Cloud Console](https://console.cloud.google.com/)でプロジェクトを作成
2. Google Docs API を有効化
3. OAuth 2.0 クライアントIDを作成（デスクトップアプリケーション）
4. リダイレクトURIに `http://localhost:8089/callback` を追加
5. クライアントIDとシークレットを取得

### 2. 環境変数の設定

```bash
export GOOGLE_CLIENT_ID="your-client-id"
export GOOGLE_CLIENT_SECRET="your-client-secret"
```

### 3. Claude Code の設定

`~/.claude-code/config.json` に以下を追加:

```json
{
  "mcpServers": {
    "google-doc-review": {
      "command": "go",
      "args": ["run", "/path/to/google-doc-review/cmd/server/main.go"],
      "env": {
        "GOOGLE_CLIENT_ID": "your-client-id",
        "GOOGLE_CLIENT_SECRET": "your-client-secret"
      }
    }
  }
}
```

## 使用方法

初回実行時にブラウザが開き、Google アカウントでの認証を求められます。
認証後、トークンは `~/.google-doc-review/token.json` に保存されます。

## 開発

### テスト実行

```bash
go test ./...
```

### モック生成

```bash
go generate ./...
```

## アーキテクチャ

- `cmd/server/main.go`: エントリーポイント
- `internal/authmanager/`: OAuth認証とトークン管理
- `internal/review/`: Google Docs取得ロジック
- `internal/mcpserver/`: MCPサーバー実装
