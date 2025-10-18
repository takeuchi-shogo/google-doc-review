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

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/docs/v1"
)

type AuthManager struct {
	config    *oauth2.Config
	tokenPath string
}

func (a *AuthManager) NewClient() (*http.Client, error) {
	context := context.Background()
	// Handle the exchange code to initiate a transport.
	tok, err := a.config.Exchange(context, "authorization-code")
	if err != nil {
		return nil, err
	}
	client := a.config.Client(context, tok)
	return client, nil
}

func New() *AuthManager {
	// 組み込みのOAuth credentials（公開アプリとして登録）
	config := &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  "http://localhost:8888/callback",
		Scopes: []string{
			docs.DocumentsReadonlyScope,
			docs.DriveReadonlyScope,
		},
		Endpoint: google.Endpoint,
	}

	return &AuthManager{
		config:    config,
		tokenPath: getTokenPath(), // ~/.design-doc-reviewer/token.json
	}
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

	fmt.Printf("ブラウザが開きます。Googleアカウントで認証してください...\n")
	fmt.Printf("開かない場合はこのURLにアクセス: %s\n", authURL)

	// ブラウザを自動で開く
	openBrowser(authURL)

	// ローカルサーバーでコールバックを待つ
	code := make(chan string)
	server := &http.Server{Addr: ":8888"}

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code <- r.URL.Query().Get("code")
		fmt.Fprintf(w, "認証成功！このウィンドウを閉じてください。")
	})

	go server.ListenAndServe()
	authCode := <-code
	server.Shutdown(context.Background())

	// トークン取得と保存
	token, err := a.config.Exchange(context.Background(), authCode)
	if err != nil {
		return err
	}

	return a.saveToken(token)
}

func (a *AuthManager) saveToken(token *oauth2.Token) error {
	json.NewEncoder(os.Stdout).Encode(token)
	return nil
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
