package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

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
	Timeout             int `yaml:"timeout"`
	MaxStories          int `yaml:"max_stories"`
	MaxTopLevelComments int `yaml:"max_top_level_comments"`
	MaxChildComments    int `yaml:"max_child_comments"`
}

type SchedulerConfig struct {
	Cron string `yaml:"cron"`
}

// findProjectRoot 查找项目根目录
// 通过查找go.mod文件来确定项目根目录
func findProjectRoot() (string, error) {
	// 获取当前执行文件的目录
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("unable to get current file path")
	}

	// 从当前文件开始向上查找
	dir := filepath.Dir(filename)
	for {
		// 检查是否存在go.mod文件
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}

		// 到达根目录
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// 如果没找到go.mod，尝试从当前工作目录开始查找
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("unable to get working directory: %w", err)
	}

	dir = wd
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("project root not found (no go.mod file)")
}

// resolveConfigPath 解析配置文件路径
// 如果是相对路径，则相对于项目根目录解析
func resolveConfigPath(configPath string) (string, error) {
	// 如果是绝对路径，直接返回
	if filepath.IsAbs(configPath) {
		return configPath, nil
	}

	// 如果文件在当前目录存在，直接使用
	if _, err := os.Stat(configPath); err == nil {
		return configPath, nil
	}

	// 查找项目根目录
	projectRoot, err := findProjectRoot()
	if err != nil {
		return "", fmt.Errorf("failed to find project root: %w", err)
	}

	// 相对于项目根目录解析路径
	resolvedPath := filepath.Join(projectRoot, configPath)

	// 检查文件是否存在
	if _, err := os.Stat(resolvedPath); err != nil {
		return "", fmt.Errorf("config file not found at %s: %w", resolvedPath, err)
	}

	return resolvedPath, nil
}

func Load(configPath string) (*Config, error) {
	// 解析配置文件路径
	resolvedPath, err := resolveConfigPath(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config path: %w", err)
	}

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", resolvedPath, err)
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
