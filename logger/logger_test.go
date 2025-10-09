package logger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"hacker-news-daily/hackernews"
)

func TestLogger_StoryContents(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "logger_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建日志配置
	config := Config{
		Enabled:          true,
		LogDir:           tempDir,
		MaxContentLength: 100,
		AsyncWrite:       false, // 同步写入便于测试
		BufferSize:       10,
	}

	// 创建日志记录器
	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// 测试数据
	date := "2023-10-09"
	stories := []hackernews.Story{
		{
			ID:    1,
			Title: "Test Story 1",
			URL:   "https://example.com/1",
		},
		{
			ID:    2,
			Title: "Test Story 2",
			URL:   "https://example.com/2",
		},
	}
	storyContents := []string{
		"This is a long content for story 1 that should be truncated because it exceeds the max content length configured in the logger settings.",
		"Short content for story 2.",
	}

	// 记录故事内容
	logger.LogStoryContents(date, stories, storyContents)

	// 检查日志文件
	logFile := filepath.Join(tempDir, "hn-daily-2023-10-09.json")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Fatalf("Log file was not created")
	}

	// 读取日志文件
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// 解析日志条目
	var entry LogEntry
	if err := json.Unmarshal(content, &entry); err != nil {
		t.Fatalf("Failed to unmarshal log entry: %v", err)
	}

	// 验证日志条目
	if entry.Type != LogTypeStoryContents {
		t.Errorf("Expected log type %s, got %s", LogTypeStoryContents, entry.Type)
	}

	if entry.Date != date {
		t.Errorf("Expected date %s, got %s", date, entry.Date)
	}

	// 验证故事内容
	var storyData StoryContentsLog
	dataBytes, err := json.Marshal(entry.Data)
	if err != nil {
		t.Fatalf("Failed to marshal data: %v", err)
	}
	if err := json.Unmarshal(dataBytes, &storyData); err != nil {
		t.Fatalf("Failed to unmarshal StoryContentsLog: %v", err)
	}

	if len(storyData.Stories) != 2 {
		t.Errorf("Expected 2 stories, got %d", len(storyData.Stories))
	}

	// 验证第一个故事被截断
	expectedLength := config.MaxContentLength + len("...[truncated]")
	if len(storyData.Stories[0].Content) != expectedLength {
		t.Errorf("Expected story content to be truncated to %d chars, got %d", expectedLength, len(storyData.Stories[0].Content))
	}

	if storyData.Stories[0].Title != "Test Story 1" {
		t.Errorf("Expected title 'Test Story 1', got '%s'", storyData.Stories[0].Title)
	}
}

func TestLogger_AISummaries(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "logger_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建日志配置
	config := Config{
		Enabled:          true,
		LogDir:           tempDir,
		MaxContentLength: 50,
		AsyncWrite:       false,
		BufferSize:       10,
	}

	// 创建日志记录器
	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// 测试数据
	date := "2023-10-09"
	summaryText := "This is a long AI summary that should be truncated because it exceeds the maximum content length configured in the logger settings."
	storyCount := 5

	// 记录AI总结
	logger.LogAISummaries(date, summaryText, storyCount)

	// 检查日志文件
	logFile := filepath.Join(tempDir, "hn-daily-2023-10-09.json")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// 解析日志条目
	var entry LogEntry
	if err := json.Unmarshal(content, &entry); err != nil {
		t.Fatalf("Failed to unmarshal log entry: %v", err)
	}

	// 验证日志条目
	if entry.Type != LogTypeAISummaries {
		t.Errorf("Expected log type %s, got %s", LogTypeAISummaries, entry.Type)
	}

	// 验证AI总结数据
	var summaryData AISummariesLog
	dataBytes, err := json.Marshal(entry.Data)
	if err != nil {
		t.Fatalf("Failed to marshal data: %v", err)
	}
	if err := json.Unmarshal(dataBytes, &summaryData); err != nil {
		t.Fatalf("Failed to unmarshal AISummariesLog: %v", err)
	}

	if summaryData.StoryCount != storyCount {
		t.Errorf("Expected story count %d, got %d", storyCount, summaryData.StoryCount)
	}

	// 验证内容被截断
	expectedLength := config.MaxContentLength + len("...[truncated]")
	if len(summaryData.RawSummaryText) != expectedLength {
		t.Errorf("Expected summary to be truncated to %d chars, got %d", expectedLength, len(summaryData.RawSummaryText))
	}
}

func TestLogger_TelegramMessage(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "logger_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建日志配置
	config := Config{
		Enabled:          true,
		LogDir:           tempDir,
		MaxContentLength: 50, // 这个不应该影响Telegram消息
		AsyncWrite:       false,
		BufferSize:       10,
	}

	// 创建日志记录器
	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// 测试数据
	date := "2023-10-09"
	title := "🗞️ Hacker News 每日热点 - 2023-10-09"
	storiesText := "[1] This is story 1 summary.\n\n[2] This is story 2 summary that is quite long and should not be truncated because Telegram messages are recorded in full."

	// 记录Telegram消息
	logger.LogTelegramMessage(date, title, storiesText)

	// 检查日志文件
	logFile := filepath.Join(tempDir, "hn-daily-2023-10-09.json")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// 解析日志条目
	var entry LogEntry
	if err := json.Unmarshal(content, &entry); err != nil {
		t.Fatalf("Failed to unmarshal log entry: %v", err)
	}

	// 验证日志条目
	if entry.Type != LogTypeTelegramMessage {
		t.Errorf("Expected log type %s, got %s", LogTypeTelegramMessage, entry.Type)
	}

	// 验证Telegram消息数据
	var telegramData TelegramMessageLog
	dataBytes, err := json.Marshal(entry.Data)
	if err != nil {
		t.Fatalf("Failed to marshal data: %v", err)
	}
	if err := json.Unmarshal(dataBytes, &telegramData); err != nil {
		t.Fatalf("Failed to unmarshal TelegramMessageLog: %v", err)
	}

	if telegramData.Title != title {
		t.Errorf("Expected title '%s', got '%s'", title, telegramData.Title)
	}

	// 验证Telegram消息没有被截断
	if telegramData.StoriesText != storiesText {
		t.Errorf("Telegram message was modified unexpectedly")
	}

	if telegramData.MessageLength != len(storiesText) {
		t.Errorf("Expected message length %d, got %d", len(storiesText), telegramData.MessageLength)
	}
}

func TestLogger_Disabled(t *testing.T) {
	// 创建禁用的日志记录器
	config := Config{
		Enabled: false,
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	if logger.IsEnabled() {
		t.Error("Logger should be disabled")
	}

	// 这些调用不应该做任何事情
	logger.LogStoryContents("2023-10-09", nil, nil)
	logger.LogAISummaries("2023-10-09", "", 0)
	logger.LogTelegramMessage("2023-10-09", "", "")

	logger.Close()
}
