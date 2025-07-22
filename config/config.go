package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AI         AIConfig         `yaml:"ai"`
	Telegram   TelegramConfig   `yaml:"telegram"`
	HackerNews HackerNewsConfig `yaml:"hacker_news"`
	Scheduler  SchedulerConfig  `yaml:"scheduler"`
}

type AIConfig struct {
	BaseURL   string `yaml:"base_url"`
	APIKey    string `yaml:"api_key"`
	Model     string `yaml:"model"`
	MaxTokens int    `yaml:"max_tokens"`
}

type TelegramConfig struct {
	BotToken string `yaml:"bot_token"`
	ChatID   string `yaml:"chat_id"`
	ProxyURL string `yaml:"proxy_url"`
}

type HackerNewsConfig struct {
	Timeout    int `yaml:"timeout"`
	MaxStories int `yaml:"max_stories"`
}

type SchedulerConfig struct {
	Cron string `yaml:"cron"`
}

func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 从环境变量覆盖敏感配置
	if apiKey := os.Getenv("AI_API_KEY"); apiKey != "" {
		config.AI.APIKey = apiKey
	}
	if botToken := os.Getenv("TELEGRAM_BOT_TOKEN"); botToken != "" {
		config.Telegram.BotToken = botToken
	}
	if chatID := os.Getenv("TELEGRAM_CHAT_ID"); chatID != "" {
		config.Telegram.ChatID = chatID
	}

	return &config, nil
}
