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
	configPath = flag.String("config", "configs/config.yaml", "配置文件路径")
	runOnce    = flag.Bool("once", false, "立即执行一次任务")
	dateFlag   = flag.String("date", "", "指定日期 (YYYY-MM-DD)，默认为今天")
)

func main() {
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化客户端
	hnClient := hackernews.NewClient(cfg.HackerNews.Timeout)
	aiClient := ai.NewClient(cfg.AI.BaseURL, cfg.AI.APIKey, cfg.AI.Model, cfg.AI.MaxTokens)
	tgBot, err := telegram.NewBot(cfg.Telegram.BotToken, cfg.Telegram.ChatID, cfg.Telegram.ProxyURL)
	if err != nil {
		log.Fatalf("Failed to create telegram bot: %v", err)
	}

	// 创建主任务
	job := func() error {
		date := *dateFlag
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}

		log.Printf("Processing Hacker News daily summary for date: %s", date)

		return processDailySummary(hnClient, aiClient, tgBot, date, cfg.HackerNews.MaxStories)
	}

	// 如果指定了立即运行，执行一次任务
	if *runOnce {
		if err := job(); err != nil {
			log.Fatalf("Job execution failed: %v", err)
		}
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

func processDailySummary(hnClient *hackernews.Client, aiClient *ai.Client, tgBot *telegram.Bot, date string, maxStories int) error {
	// 1. 获取热门故事
	log.Printf("Fetching top stories for %s...", date)
	stories, err := hnClient.GetTopStoriesByDate(date, maxStories)
	if err != nil {
		return fmt.Errorf("failed to get top stories: %w", err)
	}

	if len(stories) == 0 {
		log.Printf("No stories found for date: %s", date)
		return nil
	}

	log.Printf("Found %d stories for %s", len(stories), date)

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

	// 3. 使用 AI 总结故事
	log.Println("Generating AI summary...")
	storySummaries, err := aiClient.SummarizeStories(storyContents, date)
	if err != nil {
		return fmt.Errorf("failed to summarize stories: %w", err)
	}

	// 4. 创建每日总结
	dailySummary, err := aiClient.CreateDailySummary(storySummaries, date)
	if err != nil {
		return fmt.Errorf("failed to create daily summary: %w", err)
	}

	// 5. 发送到 Telegram
	log.Println("Sending summary to Telegram...")
	if err := tgBot.SendDailySummary(date, dailySummary); err != nil {
		// 如果发送失败，尝试发送错误信息
		if sendErr := tgBot.SendError(fmt.Sprintf("发送每日总结失败: %v", err)); sendErr != nil {
			log.Printf("Failed to send error message: %v", sendErr)
		}
		return fmt.Errorf("failed to send summary to telegram: %w", err)
	}

	log.Printf("Successfully processed and sent daily summary for %s", date)
	return nil
}
