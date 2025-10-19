package config

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		configFile  string
		envVars     map[string]string
		wantConfig  *Config
		wantErr     bool
		errContains string
	}{
		{
			name:       "load from .env.test file",
			configFile: "../.env.test",
			wantConfig: &Config{
				Google: GoogleConfig{
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
				},
			},
			wantErr: false,
		},
		{
			name:       "environment variables override file",
			configFile: "../.env.test",
			envVars: map[string]string{
				"GOOGLE_CLIENT_ID":     "env-client-id",
				"GOOGLE_CLIENT_SECRET": "env-client-secret",
			},
			wantConfig: &Config{
				Google: GoogleConfig{
					ClientID:     "env-client-id",
					ClientSecret: "env-client-secret",
				},
			},
			wantErr: false,
		},
		{
			name:       "missing client ID",
			configFile: "nonexistent.env",
			envVars: map[string]string{
				"GOOGLE_CLIENT_SECRET": "secret",
			},
			wantErr:     true,
			errContains: "GOOGLE_CLIENT_ID is required",
		},
		{
			name:       "missing client secret",
			configFile: "nonexistent.env",
			envVars: map[string]string{
				"GOOGLE_CLIENT_ID": "client-id",
			},
			wantErr:     true,
			errContains: "GOOGLE_CLIENT_SECRET is required",
		},
		{
			name:        "missing both credentials",
			configFile:  "nonexistent.env",
			wantErr:     true,
			errContains: "GOOGLE_CLIENT_ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 環境変数をクリア
			os.Unsetenv("GOOGLE_CLIENT_ID")
			os.Unsetenv("GOOGLE_CLIENT_SECRET")

			// テスト用の環境変数を設定
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}
			defer func() {
				for key := range tt.envVars {
					os.Unsetenv(key)
				}
			}()

			// 設定を読み込む
			got, err := LoadFromFile(tt.configFile)

			// エラーチェック
			if tt.wantErr {
				if err == nil {
					t.Error("LoadFromFile() expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("LoadFromFile() error = %v, should contain %v", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("LoadFromFile() unexpected error = %v", err)
				return
			}

			// 設定値を比較
			if diff := cmp.Diff(tt.wantConfig, got); diff != "" {
				t.Errorf("LoadFromFile() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLoadDefault(t *testing.T) {
	// 環境変数を設定
	os.Setenv("GOOGLE_CLIENT_ID", "default-client-id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "default-client-secret")
	defer func() {
		os.Unsetenv("GOOGLE_CLIENT_ID")
		os.Unsetenv("GOOGLE_CLIENT_SECRET")
	}()

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error = %v", err)
	}

	want := &Config{
		Google: GoogleConfig{
			ClientID:     "default-client-id",
			ClientSecret: "default-client-secret",
		},
	}

	if diff := cmp.Diff(want, config); diff != "" {
		t.Errorf("Load() mismatch (-want +got):\n%s", diff)
	}
}

func TestGoogleConfig(t *testing.T) {
	config := GoogleConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	if config.ClientID != "test-id" {
		t.Errorf("ClientID = %v, want test-id", config.ClientID)
	}

	if config.ClientSecret != "test-secret" {
		t.Errorf("ClientSecret = %v, want test-secret", config.ClientSecret)
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && hasSubstring(s, substr)))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
