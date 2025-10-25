# コメント作成（アンカー付き）

## 概要

Google Docsのドキュメントにプログラマティックにアンカー付きコメントを作成する試みと、その制約についての調査結果をまとめる。

## 背景

Google Docsのレビュー機能を自動化するため、特定のテキスト位置にアンカーされたコメントをAPIで作成しようと試みた。UIでは簡単にできる操作だが、API経由での実現には大きな制約があることが判明した。

## 実装の試行

### 試行1: Line Number方式

**アプローチ:**

```go
anchor := map[string]any{
    "region": map[string]any{
        "kind": "drive#commentRegion",
        "line": lineNumber,
        "rev":  "head",
    },
}
```

**結果:**

- Drive APIでコメントは作成されるが、Google Docs UIでは「元のコンテンツは削除されました」と表示される
- アンカーが正しく機能しない

### 試行2: Text Position方式（StartIndex/EndIndex）

**アプローチ:**

```go
// テキストの実際の位置を検索
pos, err := FindTextPosition(ctx, docID, quotedText)

anchor := map[string]any{
    "region": map[string]any{
        "startIndex": pos.StartIndex,
        "endIndex":   pos.EndIndex,
    },
}
```

**結果:**

- アンカーJSONは正しく生成される（例: `{"region":{"startIndex":14,"endIndex":44}}`）
- Drive APIのレスポンスではアンカー情報が返ってくる
- しかし、Google Docs UIではアンカーとして機能せず、コメントが適切に表示されない

### 試行3: QuotedText自動アンカー方式

**アプローチ:**

```go
comment := &drive.Comment{
    Content: content,
    QuotedFileContent: &drive.CommentQuotedFileContent{
        MimeType: "text/plain",
        Value:    quotedText,
    },
}

// FindTextPositionで位置を検索してanchorを設定
```

**結果:**

- コメントは作成されるが、やはりアンカーは機能しない

## 調査結果

### Google公式ドキュメントの記載

#### Drive API - Manage Comments

- URL: <https://developers.google.com/drive/api/guides/manage-comments>
- 重要な記載:
  > "The anchoring feature is intended for non-Google Docs editors files, not for Google Editors files (documents, presentations, spreadsheets)."

#### 制約事項

1. **アンカー機能はGoogle Editorsファイル非対応**: Drive APIの`anchor`フィールドは、Google Docs/Sheets/Slidesなどの**Google Editorsファイルには対応していない**
2. **アンカーの保証なし**: ドキュメントのリビジョン間でアンカーの位置が保証されない
3. **カスタムアンカーの制限**: Google Workspace Editorsは、カスタムアンカーを「アンカーなしコメント」として扱う可能性がある
4. **推奨用途**: 画像ファイルや読み取り専用ドキュメントなど、内容が変更されないファイルでの使用が推奨される

### コミュニティとIssue Tracker

#### Stack Overflow

- URL: <https://stackoverflow.com/questions/23498275/creating-anchored-comments-programmatically-in-google-docs>
- 2014年から報告されている問題で、現在も未解決

#### Google Issue Tracker

- Issue #36763384: "Provide ability to create a Drive API Comment anchor resource"
- URL: <https://issuetracker.google.com/issues/36763384>
- ステータス: **未解決**
- 提案: Selection classに`kix`アンカーを作成するメソッドを追加する
- 現状: Drive API Commentsエンドポイントを使ってDocsエディタ内のユーザー選択範囲にコメントをプログラマティックに接続する方法がない

### Web検索結果（2025年1月時点）

- 2025年1月時点でも開発者コミュニティで解決策を探している投稿が見つかる
- 10年以上経過しても、この機能は実装されていない

## 結論

### できること ✅

1. **アンカーなしコメントの作成**: ドキュメント全体に対するコメント
2. **QuotedTextの表示**: コメント内でテキストを引用表示（ただしアンカーはされない）
3. **コメントの一覧取得、削除**: 既存コメントの管理操作

### できないこと ❌

1. **特定位置へのアンカー付きコメント**: Google Docs UI上で特定のテキスト範囲にアンカーされたコメントの作成
2. **Line Number指定でのアンカー**: 行番号ベースのアンカー設定
3. **Text Position指定でのアンカー**: StartIndex/EndIndexでの正確なアンカー設定

### 代替案

#### 1. Suggestion Mode（提案モード）の利用

Drive APIやDocs APIで**Suggestion Mode**を使用すれば、テキストの変更を「提案」として埋め込める可能性がある。これはコメントとは異なるが、レビューワークフローには有用かもしれない。

**要調査:**

- Docs APIのSuggestion機能
- `SuggestedTextStyle`や`SuggestedParagraphStyle`の利用

#### 2. 手動でのコメント追加

API経由でコメント候補を生成し、レビュー内容をドキュメントやメールで共有して、ユーザーに手動でコメント追加してもらう。

#### 3. 別ツールでのレビュー管理

Google Docs以外のレビューツール（GitHub、Notion、Confluenceなど）でレビューコメントを管理する。

#### 4. Google Apps Scriptの利用

Apps Scriptでも同様の制限があるが、UI拡張やカスタムサイドバーでレビュー体験を改善できる可能性がある。

## 技術的詳細

### 実装したコード

**CommentManager実装箇所:**

- `internal/comment/comment.go`
  - `CreateComment()`: 基本的なコメント作成
  - `CreateAnchoredComment()`: アンカー付きコメント作成（ただし機能しない）
  - `FindTextPosition()`: テキスト位置検索
  - `CreateCommentsFromIssues()`: Issue形式からコメント作成

**MCPツール:**

- `create_comment`: アンカーなしコメント作成
- `create_anchored_comment`: アンカー付きコメント作成（制約あり）

### 実験用スクリプト

以下のスクリプトで実際に試すことができる:

```bash
# ドキュメント取得
go run scripts/fetch-doc/main.go

# 基本的なコメント作成
go run scripts/create-comments/main.go

# アンカー付きコメント作成（アンカーは機能しない）
go run scripts/create-comments-v2/main.go
```

## 推奨事項

1. **現時点では実用的でない**: Google Docs APIでのアンカー付きコメント作成は、UIでの動作を期待するなら実用的ではない
2. **Feature Requestへの投票**: [Issue #36763384](https://issuetracker.google.com/issues/36763384)にスターを付けて、Googleにニーズを伝える
3. **代替ワークフローの検討**: Suggestion ModeやApps Scriptなど、別のアプローチを検討する
4. **ドキュメント化**: この制約を明確にドキュメント化し、ユーザーに期待値を設定する

## 参考リンク

- [Drive API - Manage Comments](https://developers.google.com/drive/api/guides/manage-comments?hl=ja)
- [Stack Overflow - Creating anchored comments programmatically](https://stackoverflow.com/questions/23498275/creating-anchored-comments-programmatically-in-google-docs)
- [Google Issue Tracker #36763384](https://issuetracker.google.com/issues/36763384)
- [Drive API - Comments Reference](https://developers.google.com/drive/api/reference/rest/v3/comments)

## 更新履歴

- 2025-10-25: 初版作成 - アンカー付きコメントの制約と調査結果をまとめ
