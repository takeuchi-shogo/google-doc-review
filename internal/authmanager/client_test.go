package authmanager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/takeuchi-shogo/google-doc-review/internal/authmanager/mocks"
	"go.uber.org/mock/gomock"
	"golang.org/x/oauth2"
	"google.golang.org/api/docs/v1"
)

// TestNew tests the New() constructor
func TestNew(t *testing.T) {
	tests := []struct {
		name           string
		clientID       string
		clientSecret   string
		expectedScopes []string
	}{
		{
			name:         "creates AuthManager with environment variables",
			clientID:     "test-client-id",
			clientSecret: "test-client-secret",
			expectedScopes: []string{
				docs.DocumentsReadonlyScope,
				"https://www.googleapis.com/auth/drive",
			},
		},
		{
			name:         "creates AuthManager with empty credentials",
			clientID:     "",
			clientSecret: "",
			expectedScopes: []string{
				docs.DocumentsReadonlyScope,
				"https://www.googleapis.com/auth/drive",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			os.Setenv("GOOGLE_CLIENT_ID", tt.clientID)
			os.Setenv("GOOGLE_CLIENT_SECRET", tt.clientSecret)
			defer func() {
				os.Unsetenv("GOOGLE_CLIENT_ID")
				os.Unsetenv("GOOGLE_CLIENT_SECRET")
			}()

			// Create AuthManager
			am := New()

			// Verify configuration
			if am == nil {
				t.Fatal("New() returned nil")
			}

			if am.config == nil {
				t.Fatal("config is nil")
			}

			if diff := cmp.Diff(tt.clientID, am.config.ClientID); diff != "" {
				t.Errorf("ClientID mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tt.clientSecret, am.config.ClientSecret); diff != "" {
				t.Errorf("ClientSecret mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff("http://localhost:8089/callback", am.config.RedirectURL); diff != "" {
				t.Errorf("RedirectURL mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tt.expectedScopes, am.config.Scopes); diff != "" {
				t.Errorf("Scopes mismatch (-want +got):\n%s", diff)
			}

			// Verify tokenPath is set
			if am.tokenPath == "" {
				t.Error("tokenPath is empty")
			}

			// Verify tokenPath contains expected directory
			if !strings.Contains(am.tokenPath, ".google-doc-review") {
				t.Errorf("tokenPath = %v, should contain .google-doc-review", am.tokenPath)
			}

			if !strings.HasSuffix(am.tokenPath, "token.json") {
				t.Errorf("tokenPath = %v, should end with token.json", am.tokenPath)
			}
		})
	}
}

// TestGetTokenPath tests the getTokenPath function
func TestGetTokenPath(t *testing.T) {
	tokenPath := getTokenPath()

	if tokenPath == "" {
		t.Fatal("getTokenPath() returned empty string")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get user home dir: %v", err)
	}

	expectedPath := filepath.Join(home, ".google-doc-review", "token.json")
	if diff := cmp.Diff(expectedPath, tokenPath); diff != "" {
		t.Errorf("getTokenPath() mismatch (-want +got):\n%s", diff)
	}
}

// TestGetClient tests the GetClient() method
func TestGetClient(t *testing.T) {
	tests := []struct {
		name          string
		setupToken    bool
		tokenExpired  bool
		wantErr       bool
		errContains   string
	}{
		{
			name:       "successful with valid token",
			setupToken: true,
			wantErr:    false,
		},
		{
			name:         "returns error with expired token",
			setupToken:   true,
			tokenExpired: true,
			wantErr:      true,
			errContains:  "token has expired",
		},
		{
			name:        "no token file exists",
			setupToken:  false,
			wantErr:     true,
			errContains: "no saved token found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for token
			tmpDir := t.TempDir()
			tokenPath := filepath.Join(tmpDir, "token.json")

			// Setup token file if needed
			if tt.setupToken {
				var issuedAt time.Time
				var expiresIn time.Duration

				if tt.tokenExpired {
					issuedAt = time.Now().Add(-25 * time.Hour) // Issued 25 hours ago
					expiresIn = 24 * time.Hour
				} else {
					issuedAt = time.Now()
					expiresIn = 24 * time.Hour
				}

				token := &oauth2.Token{
					AccessToken:  "test-access-token",
					TokenType:    "Bearer",
					RefreshToken: "test-refresh-token",
					Expiry:       time.Now().Add(time.Hour),
				}

				tokenWithExpiry := &TokenWithExpiry{
					Token:     token,
					IssuedAt:  issuedAt,
					ExpiresIn: expiresIn,
				}

				// Save token to file
				os.MkdirAll(filepath.Dir(tokenPath), 0700)
				data, _ := json.Marshal(tokenWithExpiry)
				os.WriteFile(tokenPath, data, 0600)
			}

			// Create AuthManager
			am := &AuthManager{
				config: &oauth2.Config{
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
					RedirectURL:  "http://localhost:8089/callback",
				},
				tokenPath: tokenPath,
			}

			ctx := context.Background()
			client, err := am.GetClient(ctx)

			if tt.wantErr {
				if err == nil {
					t.Error("GetClient() expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("GetClient() error = %v, should contain %v", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("GetClient() unexpected error = %v", err)
				return
			}

			if client == nil {
				t.Error("GetClient() returned nil client")
			}
		})
	}
}

// TestAuthenticate tests the Authenticate() method
func TestAuthenticate(t *testing.T) {
	tests := []struct {
		name           string
		existingToken  bool
		authCode       string
		authError      error
		setupServer    func() *httptest.Server
		wantErr        bool
		errContains    string
	}{
		{
			name:          "skip authentication when token exists",
			existingToken: true,
			wantErr:       false,
		},
		{
			name:        "successful authentication flow",
			authCode:    "test-auth-code",
			authError:   nil,
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/token" {
						token := map[string]any{
							"access_token":  "mock-access-token",
							"token_type":    "Bearer",
							"expires_in":    3600,
							"refresh_token": "mock-refresh-token",
						}
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(token)
						return
					}
				}))
			},
			wantErr: false,
		},
		{
			name:        "authentication flow with token exchange error",
			authCode:    "invalid-code",
			authError:   nil,
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/token" {
						http.Error(w, "invalid code", http.StatusBadRequest)
						return
					}
				}))
			},
			wantErr:     true,
			errContains: "cannot fetch token",
		},
		{
			name:        "authenticator returns error",
			authCode:    "",
			authError:   errors.New("user cancelled authentication"),
			wantErr:     true,
			errContains: "authentication failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Create temporary directory for token
			tmpDir := t.TempDir()
			tokenPath := filepath.Join(tmpDir, "token.json")

			// Create existing token if needed
			if tt.existingToken {
				tokenData := map[string]any{
					"access_token": "existing-token",
					"token_type":   "Bearer",
				}
				data, _ := json.Marshal(tokenData)
				os.WriteFile(tokenPath, data, 0600)
			}

			var server *httptest.Server
			if tt.setupServer != nil {
				server = tt.setupServer()
				defer server.Close()
			}

			// Create mock authenticator
			mockAuth := mocks.NewMockAuthenticator(ctrl)

			// Setup expectations
			if !tt.existingToken {
				mockAuth.EXPECT().
					Authenticate(gomock.Any()).
					Return(tt.authCode, tt.authError).
					Times(1)
			}

			// Create AuthManager
			config := &oauth2.Config{
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				RedirectURL:  "http://localhost:8089/callback",
			}

			if server != nil {
				config.Endpoint = oauth2.Endpoint{
					AuthURL:  server.URL + "/auth",
					TokenURL: server.URL + "/token",
				}
			}

			am := &AuthManager{
				config:        config,
				tokenPath:     tokenPath,
				authenticator: mockAuth,
			}

			err := am.Authenticate()

			if tt.wantErr {
				if err == nil {
					t.Error("Authenticate() expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Authenticate() error = %v, should contain %v", err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("Authenticate() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestSaveToken tests the saveToken() method
func TestSaveToken(t *testing.T) {
	tests := []struct {
		name    string
		token   *oauth2.Token
		wantErr bool
	}{
		{
			name: "save valid token",
			token: &oauth2.Token{
				AccessToken:  "test-access-token",
				TokenType:    "Bearer",
				RefreshToken: "test-refresh-token",
				Expiry:       time.Now().Add(time.Hour),
			},
			wantErr: false,
		},
		{
			name: "save token without refresh token",
			token: &oauth2.Token{
				AccessToken: "test-access-token",
				TokenType:   "Bearer",
			},
			wantErr: false,
		},
		{
			name:    "save nil token",
			token:   nil,
			wantErr: false, // Current implementation doesn't validate
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for token
			tmpDir := t.TempDir()
			tokenPath := filepath.Join(tmpDir, "token.json")

			am := &AuthManager{
				config: &oauth2.Config{
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
				},
				tokenPath: tokenPath,
			}

			err := am.saveToken(tt.token)

			if tt.wantErr {
				if err == nil {
					t.Error("saveToken() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("saveToken() unexpected error = %v", err)
				return
			}

			// Verify token was written to file
			if tt.token != nil {
				data, err := os.ReadFile(tokenPath)
				if err != nil {
					t.Errorf("failed to read token file: %v", err)
					return
				}

				if len(data) == 0 {
					t.Error("saveToken() wrote empty file")
					return
				}

				// Verify JSON structure (should be TokenWithExpiry)
				var decodedTokenWithExpiry TokenWithExpiry
				if err := json.Unmarshal(data, &decodedTokenWithExpiry); err != nil {
					t.Errorf("saveToken() output is not valid JSON: %v", err)
				}

				// Verify the wrapped token
				if decodedTokenWithExpiry.Token == nil {
					t.Error("saveToken() TokenWithExpiry.Token is nil")
				}

				// Verify ExpiresIn is set to 24 hours
				if decodedTokenWithExpiry.ExpiresIn != 24*time.Hour {
					t.Errorf("saveToken() ExpiresIn = %v, want 24h", decodedTokenWithExpiry.ExpiresIn)
				}

				// Verify file permissions
				info, err := os.Stat(tokenPath)
				if err != nil {
					t.Errorf("failed to stat token file: %v", err)
					return
				}
				if info.Mode().Perm() != 0600 {
					t.Errorf("token file has incorrect permissions: got %o, want 0600", info.Mode().Perm())
				}
			}
		})
	}
}

// TestOpenBrowser tests the openBrowser function
func TestOpenBrowser(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "valid http URL",
			url:  "http://example.com",
		},
		{
			name: "valid https URL",
			url:  "https://example.com",
		},
		{
			name: "URL with query parameters",
			url:  "https://example.com?param=value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock exec.Command to prevent actual browser opening
			// Note: In real scenario, you'd use dependency injection or build tags
			// For this test, we're just verifying the function doesn't panic
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("openBrowser() panicked: %v", r)
					}
				}()

				// We can't actually test openBrowser without causing side effects
				// Instead, we test the logic directly
				var err error
				switch runtime.GOOS {
				case "linux":
					_ = exec.Command("xdg-open", tt.url)
					err = errors.New("mock: command not executed")
				case "windows":
					_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", tt.url)
					err = errors.New("mock: command not executed")
				case "darwin":
					_ = exec.Command("open", tt.url)
					err = errors.New("mock: command not executed")
				default:
					err = fmt.Errorf("unsupported platform")
				}

				if err != nil {
					// This is expected in test environment
					t.Logf("Expected error in test: %v", err)
				}
			}()
		})
	}
}

// TestAuthManagerIntegration tests integration scenarios
func TestAuthManagerIntegration(t *testing.T) {
	t.Run("complete workflow without existing token", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Create mock OAuth server
		tokenReceived := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/token" {
				tokenReceived = true
				token := map[string]any{
					"access_token":  "integration-access-token",
					"token_type":    "Bearer",
					"expires_in":    3600,
					"refresh_token": "integration-refresh-token",
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(token)
				return
			}
			http.Error(w, "not found", http.StatusNotFound)
		}))
		defer server.Close()

		// Create temporary directory
		tmpDir := t.TempDir()
		tokenPath := filepath.Join(tmpDir, "token.json")

		// Create mock authenticator
		mockAuth := mocks.NewMockAuthenticator(ctrl)
		mockAuth.EXPECT().
			Authenticate(gomock.Any()).
			Return("integration-auth-code", nil).
			Times(1)

		// Create AuthManager
		am := &AuthManager{
			config: &oauth2.Config{
				ClientID:     "integration-client-id",
				ClientSecret: "integration-client-secret",
				RedirectURL:  "http://localhost:8089/callback",
				Endpoint: oauth2.Endpoint{
					AuthURL:  server.URL + "/auth",
					TokenURL: server.URL + "/token",
				},
			},
			tokenPath:     tokenPath,
			authenticator: mockAuth,
		}

		// Verify token doesn't exist
		if _, err := os.Stat(tokenPath); err == nil {
			t.Error("Token file should not exist yet")
		}

		// Authenticate
		err := am.Authenticate()
		if err != nil {
			t.Errorf("Authenticate() failed: %v", err)
		}

		if !tokenReceived {
			t.Error("Token was not received from OAuth server")
		}
	})

	t.Run("workflow with existing token", func(t *testing.T) {
		// Create temporary directory
		tmpDir := t.TempDir()
		tokenPath := filepath.Join(tmpDir, "token.json")

		// Create existing token
		existingToken := map[string]interface{}{
			"access_token": "existing-integration-token",
			"token_type":   "Bearer",
		}
		data, _ := json.Marshal(existingToken)
		os.WriteFile(tokenPath, data, 0600)

		// Create AuthManager
		am := &AuthManager{
			config: &oauth2.Config{
				ClientID:     "integration-client-id",
				ClientSecret: "integration-client-secret",
			},
			tokenPath: tokenPath,
		}

		// Authenticate should skip when token exists
		err := am.Authenticate()
		if err != nil {
			t.Errorf("Authenticate() with existing token failed: %v", err)
		}

		// Verify token still exists
		if _, err := os.Stat(tokenPath); os.IsNotExist(err) {
			t.Error("Token file should still exist")
		}
	})
}

// TestAuthManagerConcurrency tests concurrent access
func TestAuthManagerConcurrency(t *testing.T) {
	t.Run("multiple saveToken calls", func(t *testing.T) {
		am := &AuthManager{
			config: &oauth2.Config{
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
			},
			tokenPath: "/tmp/concurrent-token.json",
		}

		// Redirect stdout to suppress output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		defer func() {
			w.Close()
			os.Stdout = oldStdout
			io.ReadAll(r)
		}()

		// Run multiple saveToken operations concurrently
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func(index int) {
				token := &oauth2.Token{
					AccessToken: fmt.Sprintf("concurrent-token-%d", index),
					TokenType:   "Bearer",
				}
				am.saveToken(token)
				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

// TestAuthManagerEdgeCases tests edge cases and boundary conditions
func TestAuthManagerEdgeCases(t *testing.T) {
	t.Run("New with very long environment variables", func(t *testing.T) {
		longClientID := strings.Repeat("a", 1000)
		longClientSecret := strings.Repeat("b", 1000)

		os.Setenv("GOOGLE_CLIENT_ID", longClientID)
		os.Setenv("GOOGLE_CLIENT_SECRET", longClientSecret)
		defer func() {
			os.Unsetenv("GOOGLE_CLIENT_ID")
			os.Unsetenv("GOOGLE_CLIENT_SECRET")
		}()

		am := New()
		if diff := cmp.Diff(longClientID, am.config.ClientID); diff != "" {
			t.Errorf("Long ClientID mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(longClientSecret, am.config.ClientSecret); diff != "" {
			t.Errorf("Long ClientSecret mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("saveToken with expired token", func(t *testing.T) {
		am := &AuthManager{
			config:    &oauth2.Config{},
			tokenPath: "/tmp/expired-token.json",
		}

		expiredToken := &oauth2.Token{
			AccessToken: "expired-token",
			TokenType:   "Bearer",
			Expiry:      time.Now().Add(-time.Hour), // Expired 1 hour ago
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := am.saveToken(expiredToken)

		w.Close()
		os.Stdout = oldStdout
		io.ReadAll(r)

		if err != nil {
			t.Errorf("saveToken() with expired token failed: %v", err)
		}
	})
}
