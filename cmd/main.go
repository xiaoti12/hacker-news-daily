package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hacker-news-daily/ai"
	config "hacker-news-daily/configs"
	"hacker-news-daily/hackernews"
	"hacker-news-daily/logger"
	"hacker-news-daily/scheduler"
	"hacker-news-daily/telegram"
)

var (
	configPath = flag.String("config", "configs/config.yaml", "配置文件路径")
	runOnce    = flag.Bool("once", false, "立即执行一次任务后退出")
	sendNow    = flag.Bool("send", false, "启动时立即发送一次消息，然后继续运行支持交互")
	dateFlag   = flag.String("date", "", "指定日期 (YYYY-MM-DD)，默认为今天")
)

func main() {
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志模块
	loggerConfig := logger.Config{
		Enabled:          cfg.Logging.Enabled,
		LogDir:           cfg.Logging.LogDir,
		MaxContentLength: cfg.Logging.MaxContentLength,
		AsyncWrite:       cfg.Logging.AsyncWrite,
		BufferSize:       cfg.Logging.BufferSize,
	}
	logInstance, err := logger.NewLogger(loggerConfig)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logInstance.Close()

	// 初始化客户端
	hnClient := hackernews.NewClient(cfg.HackerNews.Timeout, cfg.HackerNews.MaxTopLevelComments, cfg.HackerNews.MaxChildComments)
	aiClient := ai.NewClient(cfg.AI.BaseURL, cfg.AI.APIKey, cfg.AI.Model, cfg.AI.MaxTokens)
	aiClient.SetLogger(logInstance)
	tgBot, err := telegram.NewBot(cfg.Telegram.BotToken, cfg.Telegram.ChatID, cfg.Telegram.ProxyURL, cfg.HackerNews.MaxStories)
	if err != nil {
		log.Fatalf("Failed to create telegram bot: %v", err)
	}

	// 设置Telegram机器人的客户端
	tgBot.SetClients(aiClient, hnClient, logInstance)

	// 启动Telegram消息处理器
	tgBot.StartMessageHandler()
	defer tgBot.StopMessageHandler()

	// 创建主任务
	job := func() error {
		date := *dateFlag
		// date为空时，GetTopStoriesByDate会自动获取过去24小时的内容

		if date == "" {
			date = time.Now().Format("2006-01-02")
			log.Printf("Processing numbered Hacker News daily summary for the last 24 hours")
		} else {
			log.Printf("Processing numbered Hacker News daily summary for date: %s", date)
		}

		return processDailySummary(tgBot, date, cfg.HackerNews.MaxStories)
	}

	// 如果指定了立即发送，执行一次带编号的消息发送
	if *sendNow {
		date := *dateFlag
		if date == "" {
			date = time.Now().Format("2006-01-02")
			log.Printf("Sending numbered Hacker News daily summary for the last 24 hours")
		} else {
			log.Printf("Sending numbered Hacker News daily summary for date: %s", date)
		}

		if err := processDailySummary(tgBot, date, cfg.HackerNews.MaxStories); err != nil {
			log.Fatalf("Send execution failed: %v", err)
		}
		log.Println("Initial numbered summary sent successfully, bot continues running for interaction...")
	}

	// 如果指定了立即运行，执行一次任务然后退出
	if *runOnce {
		if err := job(); err != nil {
			log.Fatalf("Job execution failed: %v", err)
		}
		log.Println("Once execution completed, exiting...")
		return
	}

	// 设置定时任务
	sched := scheduler.NewScheduler()
	if err := sched.AddJob(cfg.Scheduler.Cron, job); err != nil {
		log.Fatalf("Failed to add scheduled job: %v", err)
	}

	sched.Start()
	defer sched.Stop()

	log.Printf("Hacker News Daily Bot started with cron: %s", cfg.Scheduler.Cron)

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
}

// processDailySummary 处理并发送带编号的每日总结
func processDailySummary(tgBot *telegram.Bot, date string, maxStories int) error {
	// 使用 bot 的 ProcessDailySummary 方法
	if err := tgBot.ProcessDailySummary(date, maxStories); err != nil {
		// 如果发送失败，尝试发送错误信息
		if sendErr := tgBot.SendError(fmt.Sprintf("发送带编号总结失败: %v", err)); sendErr != nil {
			log.Printf("Failed to send error message: %v", sendErr)
		}
		return fmt.Errorf("failed to process daily summary: %w", err)
	}

	return nil
}
