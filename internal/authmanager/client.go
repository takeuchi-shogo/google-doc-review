package authmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/docs/v1"
)

// TokenWithExpiry wraps oauth2.Token with custom expiration time
type TokenWithExpiry struct {
	Token     *oauth2.Token `json:"token"`
	IssuedAt  time.Time     `json:"issued_at"`
	ExpiresIn time.Duration `json:"expires_in"`
}

// IsExpired checks if the token has expired based on custom expiration time
func (t *TokenWithExpiry) IsExpired() bool {
	return time.Since(t.IssuedAt) > t.ExpiresIn
}

// Authenticator handles the OAuth authentication flow
//
//go:generate mockgen -destination=mocks/mock_authenticator.go -package=mocks github.com/takeuchi-shogo/google-doc-review/internal/authmanager Authenticator
type Authenticator interface {
	// Authenticate performs the OAuth flow and returns the authorization code
	Authenticate(authURL string) (string, error)
}

type AuthManager struct {
	config        *oauth2.Config
	tokenPath     string
	authenticator Authenticator
}

// GetClient returns an authenticated HTTP client using saved token
// Returns error if token doesn't exist or is expired
func (a *AuthManager) GetClient(ctx context.Context) (*http.Client, error) {
	// トークンを読み込む
	tokenWithExpiry, err := a.loadToken()
	if err != nil {
		return nil, fmt.Errorf("no saved token found: %w", err)
	}

	// 有効期限チェック
	if tokenWithExpiry.IsExpired() {
		// 期限切れの場合はトークンファイルを削除
		os.Remove(a.tokenPath)
		return nil, fmt.Errorf("token has expired after %v, please re-authenticate", tokenWithExpiry.ExpiresIn)
	}

	// 認証済みクライアントを作成
	client := a.config.Client(ctx, tokenWithExpiry.Token)
	return client, nil
}

// GetOrAuthenticateClient returns an authenticated HTTP client
// If token doesn't exist, it will trigger authentication flow
func (a *AuthManager) GetOrAuthenticateClient(ctx context.Context) (*http.Client, error) {
	// まず既存のトークンで試す
	client, err := a.GetClient(ctx)
	if err == nil {
		return client, nil
	}

	// トークンが存在しない場合は認証を実行
	if err := a.Authenticate(); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// 認証後にクライアントを取得
	return a.GetClient(ctx)
}

// BrowserAuthenticator implements Authenticator using browser-based OAuth flow
type BrowserAuthenticator struct{}

func (b *BrowserAuthenticator) Authenticate(authURL string) (string, error) {
	fmt.Printf("ブラウザが開きます。Googleアカウントで認証してください...\n")
	fmt.Printf("開かない場合はこのURLにアクセス: %s\n", authURL)

	// ブラウザを自動で開く
	openBrowser(authURL)

	// ローカルサーバーでコールバックを待つ
	code := make(chan string)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code <- r.URL.Query().Get("code")
		fmt.Fprintf(w, "認証成功！このウィンドウを閉じてください。")
	})

	server := &http.Server{Addr: ":8089", Handler: mux}

	go server.ListenAndServe()
	authCode := <-code
	server.Shutdown(context.Background())

	return authCode, nil
}

func New() *AuthManager {
	return NewWithAuthenticator(&BrowserAuthenticator{})
}

func NewWithConfig(clientID, clientSecret string, authenticator Authenticator) *AuthManager {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  "http://localhost:8089/callback",
		Scopes: []string{
			docs.DocumentsReadonlyScope,
			docs.DriveReadonlyScope,
		},
		Endpoint: google.Endpoint,
	}

	return &AuthManager{
		config:        config,
		tokenPath:     getTokenPath(),
		authenticator: authenticator,
	}
}

func NewWithAuthenticator(authenticator Authenticator) *AuthManager {
	// 組み込みのOAuth credentials（公開アプリとして登録）
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	// デバッグ用: 環境変数が設定されているか確認
	if clientID == "" {
		fmt.Fprintf(os.Stderr, "WARNING: GOOGLE_CLIENT_ID is not set\n")
	}
	if clientSecret == "" {
		fmt.Fprintf(os.Stderr, "WARNING: GOOGLE_CLIENT_SECRET is not set\n")
	}

	return NewWithConfig(clientID, clientSecret, authenticator)
}

func getTokenPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	return filepath.Join(home, ".google-doc-review", "token.json")
}

// 初回認証フロー（自動でブラウザを開く）
func (a *AuthManager) Authenticate() error {
	// トークンが既に存在すればスキップ
	if _, err := os.Stat(a.tokenPath); err == nil {
		return nil
	}

	// OAuth フロー開始
	authURL := a.config.AuthCodeURL("state", oauth2.AccessTypeOffline)

	// Authenticatorを使って認証コードを取得
	authCode, err := a.authenticator.Authenticate(authURL)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// トークン取得と保存
	token, err := a.config.Exchange(context.Background(), authCode)
	if err != nil {
		return err
	}

	return a.saveToken(token)
}

func (a *AuthManager) saveToken(token *oauth2.Token) error {
	// ディレクトリを作成（存在しない場合）
	dir := filepath.Dir(a.tokenPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}

	// TokenWithExpiryを作成（デフォルト24時間）
	tokenWithExpiry := &TokenWithExpiry{
		Token:     token,
		IssuedAt:  time.Now(),
		ExpiresIn: 24 * time.Hour,
	}

	// トークンをJSONに変換
	data, err := json.Marshal(tokenWithExpiry)
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	// ファイルに保存（所有者のみ読み書き可能）
	if err := os.WriteFile(a.tokenPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	return nil
}

func (a *AuthManager) loadToken() (*TokenWithExpiry, error) {
	// ファイルを読み込む
	data, err := os.ReadFile(a.tokenPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	// JSONをパース
	var tokenWithExpiry TokenWithExpiry
	if err := json.Unmarshal(data, &tokenWithExpiry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	return &tokenWithExpiry, nil
}

// openBrowser opens the default browser to the specified URL
func openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	if err != nil {
		log.Printf("Failed to open browser: %v", err)
		fmt.Printf("Please open the following URL in your browser: %s\n", url)
	}
}
