package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	AI         AIConfig         `mapstructure:"ai"`
	Telegram   TelegramConfig   `mapstructure:"telegram"`
	HackerNews HackerNewsConfig `mapstructure:"hacker_news"`
	Scheduler  SchedulerConfig  `mapstructure:"scheduler"`
	Logging    LoggingConfig    `mapstructure:"logging"`
}

// 全局配置实例和互斥锁
var (
	globalConfig *Config
	configMutex  sync.RWMutex
)

type AIConfig struct {
	BaseURL   string `mapstructure:"base_url"`
	APIKey    string `mapstructure:"api_key"`
	Model     string `mapstructure:"model"`
	MaxTokens int    `mapstructure:"max_tokens"`
}

type TelegramConfig struct {
	BotToken string `mapstructure:"bot_token"`
	ChatID   string `mapstructure:"chat_id"`
	ProxyURL string `mapstructure:"proxy_url"`
}

type HackerNewsConfig struct {
	Timeout             int `mapstructure:"timeout"`
	MaxStories          int `mapstructure:"max_stories"`
	MaxTopLevelComments int `mapstructure:"max_top_level_comments"`
	MaxChildComments    int `mapstructure:"max_child_comments"`
}

type SchedulerConfig struct {
	Cron string `mapstructure:"cron"`
}

type LoggingConfig struct {
	Enabled          bool   `mapstructure:"enabled"`
	LogDir           string `mapstructure:"log_dir"`
	MaxContentLength int    `mapstructure:"max_content_length"`
	AsyncWrite       bool   `mapstructure:"async_write"`
	BufferSize       int    `mapstructure:"buffer_size"`
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

// Load 加载配置文件并启动热加载监听
func Load(configPath string) (*Config, error) {
	// 解析配置文件路径
	resolvedPath, err := resolveConfigPath(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config path: %w", err)
	}

	// 创建viper实例
	v := viper.New()

	// 设置配置文件路径
	v.SetConfigFile(resolvedPath)

	// 设置环境变量支持
	v.AutomaticEnv()
	v.SetEnvPrefix("HND") // Hacker News Daily

	// 设置环境变量映射
	v.BindEnv("ai.api_key", "AI_API_KEY")
	v.BindEnv("telegram.bot_token", "TELEGRAM_BOT_TOKEN")
	v.BindEnv("telegram.chat_id", "TELEGRAM_CHAT_ID")

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析配置到结构体
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 设置全局配置
	configMutex.Lock()
	globalConfig = &config
	configMutex.Unlock()

	// 启动配置文件热加载监听
	go watchConfig(v)

	return &config, nil
}

// GetConfig 获取当前配置（线程安全）
func GetConfig() *Config {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return globalConfig
}

// watchConfig 监听配置文件变化并重新加载
func watchConfig(v *viper.Viper) {
	// 设置配置文件变化回调
	v.OnConfigChange(func(e fsnotify.Event) {
		log.Printf("Config file changed: %s", e.Name)

		// 重新读取配置
		if err := v.ReadInConfig(); err != nil {
			log.Printf("Failed to reload config: %v", err)
			return
		}

		// 解析新的配置
		var newConfig Config
		if err := v.Unmarshal(&newConfig); err != nil {
			log.Printf("Failed to unmarshal reloaded config: %v", err)
			return
		}

		// 更新全局配置
		configMutex.Lock()
		globalConfig = &newConfig
		configMutex.Unlock()

		log.Println("Configuration reloaded successfully")
	})

	// 开始监听配置文件变化
	v.WatchConfig()
}
