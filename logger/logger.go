package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"hacker-news-daily/hackernews"
)

// LogType 日志类型
type LogType string

const (
	LogTypeStoryContents   LogType = "story_contents"
	LogTypeAISummaries     LogType = "ai_summaries"
	LogTypeTelegramMessage LogType = "telegram_message"
)

// LogEntry 日志条目
type LogEntry struct {
	Timestamp time.Time   `json:"timestamp"`
	Type      LogType     `json:"type"`
	Date      string      `json:"date"`
	Data      interface{} `json:"data"`
}

// StoryContentsLog 故事内容日志
type StoryContentsLog struct {
	Stories []StoryContentLog `json:"stories"`
}

// StoryContentLog 单个故事内容日志
type StoryContentLog struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

// AISummariesLog AI总结日志
type AISummariesLog struct {
	RawSummaryText string `json:"raw_summary_text"`
	StoryCount     int    `json:"story_count"`
}

// TelegramMessageLog Telegram消息日志
type TelegramMessageLog struct {
	Title         string `json:"title"`
	StoriesText   string `json:"stories_text"`
	MessageLength int    `json:"message_length"`
}

// Config 日志配置
type Config struct {
	Enabled          bool
	LogDir           string
	MaxContentLength int
	AsyncWrite       bool
	BufferSize       int
}

// Logger 日志记录器
type Logger struct {
	config    Config
	file      *os.File
	buffer    chan LogEntry
	wg        sync.WaitGroup
	mu        sync.RWMutex
	stopChan  chan struct{}
	isRunning bool
}

// NewLogger 创建新的日志记录器
func NewLogger(config Config) (*Logger, error) {
	if !config.Enabled {
		return &Logger{config: config}, nil
	}

	// 确保日志目录存在
	if err := os.MkdirAll(config.LogDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	logger := &Logger{
		config:    config,
		buffer:    make(chan LogEntry, config.BufferSize),
		stopChan:  make(chan struct{}),
		isRunning: true,
	}

	// 启动异步写入协程
	if config.AsyncWrite {
		logger.wg.Add(1)
		go logger.asyncWriter()
	}

	return logger, nil
}

// IsEnabled 检查日志是否启用
func (l *Logger) IsEnabled() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.config.Enabled && l.isRunning
}

// LogStoryContents 记录故事内容
func (l *Logger) LogStoryContents(date string, stories []hackernews.Story, storyContents []string) {
	if !l.IsEnabled() {
		return
	}

	logData := StoryContentsLog{
		Stories: make([]StoryContentLog, 0, len(stories)),
	}

	for i, story := range stories {
		if i >= len(storyContents) {
			break
		}

		content := storyContents[i]
		if len(content) > l.config.MaxContentLength {
			content = content[:l.config.MaxContentLength] + "...[truncated]"
		}

		logData.Stories = append(logData.Stories, StoryContentLog{
			ID:      story.ID,
			Title:   story.Title,
			URL:     story.URL,
			Content: content,
		})
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Type:      LogTypeStoryContents,
		Date:      date,
		Data:      logData,
	}

	l.writeEntry(entry)
}

// LogAISummaries 记录AI总结
func (l *Logger) LogAISummaries(date string, summaryText string, storyCount int) {
	if !l.IsEnabled() {
		return
	}

	content := summaryText
	if len(content) > l.config.MaxContentLength {
		content = content[:l.config.MaxContentLength] + "...[truncated]"
	}

	logData := AISummariesLog{
		RawSummaryText: content,
		StoryCount:     storyCount,
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Type:      LogTypeAISummaries,
		Date:      date,
		Data:      logData,
	}

	l.writeEntry(entry)
}

// LogTelegramMessage 记录Telegram消息
func (l *Logger) LogTelegramMessage(date string, title string, storiesText string) {
	if !l.IsEnabled() {
		return
	}

	// Telegram消息不截断
	logData := TelegramMessageLog{
		Title:         title,
		StoriesText:   storiesText,
		MessageLength: len(storiesText),
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Type:      LogTypeTelegramMessage,
		Date:      date,
		Data:      logData,
	}

	l.writeEntry(entry)
}

// writeEntry 写入日志条目
func (l *Logger) writeEntry(entry LogEntry) {
	if l.config.AsyncWrite {
		select {
		case l.buffer <- entry:
		default:
			// 缓冲区满，直接丢弃或同步写入
			// 这里选择丢弃，避免阻塞主流程
		}
	} else {
		l.writeToFile(entry)
	}
}

// writeToFile 写入文件
func (l *Logger) writeToFile(entry LogEntry) error {
	filename := l.getLogFileNameForDate(entry.Date)

	// 打开文件（追加模式）
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// 序列化为JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	// 写入文件
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write log entry: %w", err)
	}

	return nil
}

// asyncWriter 异步写入协程
func (l *Logger) asyncWriter() {
	defer l.wg.Done()

	for {
		select {
		case entry := <-l.buffer:
			if err := l.writeToFile(entry); err != nil {
				// 记录错误到标准输出，避免递归调用
				fmt.Printf("Failed to write log entry: %v\n", err)
			}
		case <-l.stopChan:
			// 处理剩余的缓冲区内容
			for len(l.buffer) > 0 {
				entry := <-l.buffer
				if err := l.writeToFile(entry); err != nil {
					fmt.Printf("Failed to write log entry: %v\n", err)
				}
			}
			return
		}
	}
}

// getLogFileName 获取日志文件名
func (l *Logger) getLogFileName() string {
	today := time.Now().Format("2006-01-02")
	return filepath.Join(l.config.LogDir, fmt.Sprintf("hn-daily-%s.json", today))
}

// getLogFileNameForDate 获取指定日期的日志文件名
func (l *Logger) getLogFileNameForDate(date string) string {
	return filepath.Join(l.config.LogDir, fmt.Sprintf("hn-daily-%s.json", date))
}

// Close 关闭日志记录器
func (l *Logger) Close() error {
	if !l.IsEnabled() {
		return nil
	}

	l.mu.Lock()
	l.isRunning = false
	l.mu.Unlock()

	if l.config.AsyncWrite {
		close(l.stopChan)
		l.wg.Wait()
	}

	return nil
}

// truncateString 截断字符串
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength] + "...[truncated]"
}

// cleanText 清理文本，移除换行符等
func cleanText(text string) string {
	// 替换换行符为空格，保持单行格式
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	// 压缩多个空格
	text = strings.Join(strings.Fields(text), " ")
	return text
}
