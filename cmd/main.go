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
	"hacker-news-daily/config"
	"hacker-news-daily/hackernews"
	"hacker-news-daily/scheduler"
	"hacker-news-daily/telegram"
)

var (
	configPath   = flag.String("config", "configs/config.yaml", "配置文件路径")
	runOnce     = flag.Bool("once", false, "立即执行一次任务后退出")
	sendNow     = flag.Bool("send", false, "启动时立即发送一次消息，然后继续运行支持交互")
	dateFlag    = flag.String("date", "", "指定日期 (YYYY-MM-DD)，默认为今天")
)

func main() {
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化客户端
	hnClient := hackernews.NewClient(cfg.HackerNews.Timeout, cfg.HackerNews.MaxTopLevelComments, cfg.HackerNews.MaxChildComments)
	aiClient := ai.NewClient(cfg.AI.BaseURL, cfg.AI.APIKey, cfg.AI.Model, cfg.AI.MaxTokens)
	tgBot, err := telegram.NewBot(cfg.Telegram.BotToken, cfg.Telegram.ChatID, cfg.Telegram.ProxyURL)
	if err != nil {
		log.Fatalf("Failed to create telegram bot: %v", err)
	}

	// 设置Telegram机器人的客户端
	tgBot.SetClients(aiClient, hnClient)

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

		return processDailySummary(hnClient, aiClient, tgBot, date, cfg.HackerNews.MaxStories)
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

		if err := processDailySummary(hnClient, aiClient, tgBot, date, cfg.HackerNews.MaxStories); err != nil {
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
func processDailySummary(hnClient *hackernews.Client, aiClient *ai.Client, tgBot *telegram.Bot, date string, maxStories int) error {
	// 1. 获取热门故事
	log.Println("Fetching top stories")

	stories, err := hnClient.GetTopStoriesByDate(date, maxStories)
	if err != nil {
		return fmt.Errorf("failed to get top stories: %w", err)
	}

	if len(stories) == 0 {
		log.Println("No stories found")
		return nil
	}

	log.Printf("Found %d top stories", len(stories))

	// 2. 获取每个故事的详细内容
	storyContents := make([]string, 0, len(stories))
	for i, story := range stories {
		log.Printf("Processing story %d/%d: %s", i+1, len(stories), story.Title)

		content, err := hnClient.GetStoryContent(story)
		if err != nil {
			log.Printf("Failed to get content for story %d: %v", story.ID, err)
			continue
		}

		storyContents = append(storyContents, content)

		// 添加延迟避免请求过快
		time.Sleep(1 * time.Second)
	}

	if len(storyContents) == 0 {
		return fmt.Errorf("no story content retrieved")
	}

	// 3. 使用 AI 生成带编号的故事总结
	log.Println("Generating AI summary with numbers...")
	dailySummaryWithNumbers, err := aiClient.SummarizeStoriesWithNumbers(storyContents, stories, date)
	if err != nil {
		return fmt.Errorf("failed to summarize stories with numbers: %w", err)
	}

	// 4. 发送到 Telegram (带编号)
	log.Println("Sending numbered summary to Telegram...")
	if err := tgBot.SendDailySummaryWithNumbers(dailySummaryWithNumbers); err != nil {
		// 如果发送失败，尝试发送错误信息
		if sendErr := tgBot.SendError(fmt.Sprintf("发送带编号总结失败: %v", err)); sendErr != nil {
			log.Printf("Failed to send error message: %v", sendErr)
		}
		return fmt.Errorf("failed to send numbered summary to telegram: %w", err)
	}

	log.Println("Successfully processed and sent numbered daily summary")
	return nil
}
