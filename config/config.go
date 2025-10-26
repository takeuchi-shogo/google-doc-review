package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Google GoogleConfig
}

type GoogleConfig struct {
	ClientID     string `mapstructure:"GOOGLE_CLIENT_ID"`
	ClientSecret string `mapstructure:"GOOGLE_CLIENT_SECRET"`
	TestDocID    string `mapstructure:"GOOGLE_TEST_DOC_ID"`
}

// Load loads configuration from .env file and environment variables
func Load() (*Config, error) {
	return LoadFromFile(".env")
}

// LoadFromFile loads configuration from specified file and environment variables
func LoadFromFile(configFile string) (*Config, error) {
	// viperをリセット
	v := viper.New()

	// 設定ファイルから読み込む
	v.SetConfigFile(configFile)
	v.SetConfigType("env")

	// ファイルが存在する場合は読み込む
	if err := v.ReadInConfig(); err != nil {
		// ファイルが存在しない場合は警告を出すが続行
		fmt.Printf("Warning: config file %s not found: %v\n", configFile, err)
	}

	// 環境変数を優先（ファイルよりも優先度が高い）
	v.AutomaticEnv()

	config := &Config{
		Google: GoogleConfig{
			ClientID:     v.GetString("GOOGLE_CLIENT_ID"),
			ClientSecret: v.GetString("GOOGLE_CLIENT_SECRET"),
			TestDocID:    v.GetString("GOOGLE_TEST_DOC_ID"),
		},
	}

	// 必須項目のバリデーション
	if config.Google.ClientID == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID is required")
	}
	if config.Google.ClientSecret == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_SECRET is required")
	}

	return config, nil
}
