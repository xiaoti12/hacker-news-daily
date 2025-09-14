package telegram

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"hacker-news-daily/ai"
	"hacker-news-daily/hackernews"
)

type Bot struct {
	api            *tgbotapi.BotAPI
	chatID         int64
	aiClient       *ai.Client
	hnClient       *hackernews.Client
	storySummaries map[string]*hackernews.DailySummaryWithNumbers // 按日期存储的故事总结
	mu             sync.RWMutex                                   // 读写锁保护共享数据
	messageHandler chan tgbotapi.Update                           // 消息处理通道
	stopHandler    chan struct{}                                  // 停止处理器通道
	maxStories     int                                            // 最大故事数量配置
}

func NewBot(token, chatIDStr, proxyURL string, maxStories int) (*Bot, error) {
	var bot *tgbotapi.BotAPI
	var err error

	// 如果配置了代理，使用代理创建 bot
	if proxyURL != "" {
		proxyURLParsed, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}

		client := &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURLParsed),
			},
		}

		bot, err = tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, client)
		if err != nil {
			return nil, fmt.Errorf("failed to create telegram bot with proxy: %w", err)
		}

		log.Printf("Telegram bot using proxy: %s", proxyURL)
	} else {
		bot, err = tgbotapi.NewBotAPI(token)
		if err != nil {
			return nil, fmt.Errorf("failed to create telegram bot: %w", err)
		}
	}

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %w", err)
	}

	log.Printf("Telegram bot authorized on account %s", bot.Self.UserName)

	return &Bot{
		api:            bot,
		chatID:         chatID,
		storySummaries: make(map[string]*hackernews.DailySummaryWithNumbers),
		messageHandler: make(chan tgbotapi.Update, 100),
		stopHandler:    make(chan struct{}),
		maxStories:     maxStories,
	}, nil
}

// SendDailySummary 发送每日总结
func (b *Bot) SendDailySummary(date, summary string) error {
	// Telegram 消息长度限制为 4096 字符
	const maxMessageLength = 4000

	title := fmt.Sprintf("🗞️ Hacker News 每日热点 - %s", date)

	// 如果消息太长，需要分割发送
	if len(summary) <= maxMessageLength-len(title)-20 {
		message := fmt.Sprintf("%s\n\n%s", title, summary)
		return b.sendMessage(message)
	}

	// 发送标题
	if err := b.sendMessage(title); err != nil {
		return err
	}

	// 分割内容发送
	return b.sendLongMessage(summary, maxMessageLength)
}

// sendMessage 发送单条消息
func (b *Bot) sendMessage(text string) error {
	msg := tgbotapi.NewMessage(b.chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.DisableWebPagePreview = true

	_, err := b.api.Send(msg)
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}

	return nil
}

// sendLongMessage 发送长消息（分割发送）
func (b *Bot) sendLongMessage(text string, maxLength int) error {
	if len(text) <= maxLength {
		return b.sendMessage(text)
	}

	// 按段落分割
	paragraphs := strings.Split(text, "\n\n")
	var currentMessage strings.Builder

	for _, paragraph := range paragraphs {
		// 如果单个段落就超过长度限制，需要进一步分割
		if len(paragraph) > maxLength {
			if currentMessage.Len() > 0 {
				if err := b.sendMessage(currentMessage.String()); err != nil {
					return err
				}
				currentMessage.Reset()
			}

			// 按句子分割长段落
			if err := b.sendVeryLongParagraph(paragraph, maxLength); err != nil {
				return err
			}
			continue
		}

		// 检查加入当前段落后是否超长
		if currentMessage.Len()+len(paragraph)+2 > maxLength {
			if currentMessage.Len() > 0 {
				if err := b.sendMessage(currentMessage.String()); err != nil {
					return err
				}
				currentMessage.Reset()
			}
		}

		if currentMessage.Len() > 0 {
			currentMessage.WriteString("\n\n")
		}
		currentMessage.WriteString(paragraph)
	}

	// 发送剩余内容
	if currentMessage.Len() > 0 {
		return b.sendMessage(currentMessage.String())
	}

	return nil
}

// sendVeryLongParagraph 发送超长段落
func (b *Bot) sendVeryLongParagraph(paragraph string, maxLength int) error {
	// 按句子分割
	sentences := strings.Split(paragraph, "。")
	var currentMessage strings.Builder

	for i, sentence := range sentences {
		if i < len(sentences)-1 {
			sentence += "。"
		}

		if currentMessage.Len()+len(sentence) > maxLength {
			if currentMessage.Len() > 0 {
				if err := b.sendMessage(currentMessage.String()); err != nil {
					return err
				}
				currentMessage.Reset()
			}
		}

		currentMessage.WriteString(sentence)
	}

	if currentMessage.Len() > 0 {
		return b.sendMessage(currentMessage.String())
	}

	return nil
}

// SendError 发送错误消息
func (b *Bot) SendError(errorMsg string) error {
	message := fmt.Sprintf("❌ 错误: %s", errorMsg)
	return b.sendMessage(message)
}

// SetClients 设置AI和Hacker News客户端
func (b *Bot) SetClients(aiClient *ai.Client, hnClient *hackernews.Client) {
	b.aiClient = aiClient
	b.hnClient = hnClient
}

// SendDailySummaryWithNumbers 发送带编号的每日总结
func (b *Bot) SendDailySummaryWithNumbers(summary *hackernews.DailySummaryWithNumbers) error {
	// 保存总结到内存中供后续查询
	b.mu.Lock()
	b.storySummaries[summary.Date] = summary
	b.mu.Unlock()

	// Telegram 消息长度限制为 4096 字符
	const maxMessageLength = 4000

	title := fmt.Sprintf("🗞️ Hacker News 每日热点 - %s\n\n💡 回复故事编号（如 1、2、3）获取详细总结", summary.Date)

	// 构建带编号的故事列表
	var storiesBuilder strings.Builder
	for _, storySummary := range summary.StorySummaries {
		storiesBuilder.WriteString(fmt.Sprintf("[%d] %s\n\n", storySummary.Number, storySummary.Summary))
	}

	storiesText := storiesBuilder.String()

	// 如果消息太长，需要分割发送
	if len(storiesText) <= maxMessageLength-len(title)-20 {
		message := fmt.Sprintf("%s\n%s", title, storiesText)
		return b.sendMessage(message)
	}

	// 发送标题
	if err := b.sendMessage(title); err != nil {
		return err
	}

	// 分割内容发送
	return b.sendLongMessage(storiesText, maxMessageLength)
}

// SendDetailedSummary 发送单个故事的详细总结
func (b *Bot) SendDetailedSummary(storyNumber int, date string) error {
	// 获取对应的故事总结
	b.mu.RLock()
	summary, exists := b.storySummaries[date]
	b.mu.RUnlock()

	if !exists {
		return fmt.Errorf("找不到 %s 的故事总结", date)
	}

	// 查找对应编号的故事
	var targetStory *hackernews.StoryWithNumber
	var targetFullStory *hackernews.Story
	for i, storySummary := range summary.StorySummaries {
		if storySummary.Number == storyNumber {
			targetStory = &summary.StorySummaries[i]
			targetFullStory = &summary.Stories[i]
			break
		}
	}

	if targetStory == nil {
		return fmt.Errorf("找不到编号为 %d 的故事", storyNumber)
	}

	// 获取故事的详细内容
	log.Printf("Fetching detailed content for story %d: %s", targetStory.StoryID, targetStory.Title)
	content, err := b.hnClient.GetStoryContent(*targetFullStory)
	if err != nil {
		return fmt.Errorf("获取故事内容失败: %w", err)
	}

	// 使用AI生成详细总结
	log.Printf("Generating detailed summary for story %d", storyNumber)
	detailedSummary, err := b.aiClient.GenerateDetailedSummary(*targetFullStory, content)
	if err != nil {
		return fmt.Errorf("生成详细总结失败: %w", err)
	}

	// 发送详细总结
	title := fmt.Sprintf("📖 故事 [%d] 详细总结 - %s", storyNumber, targetStory.Title)

	// 如果消息太长，分割发送
	const maxMessageLength = 4000
	if len(detailedSummary) <= maxMessageLength-len(title)-20 {
		message := fmt.Sprintf("%s\n\n%s", title, detailedSummary)
		return b.sendMessage(message)
	}

	// 发送标题
	if err := b.sendMessage(title); err != nil {
		return err
	}

	// 分割内容发送
	return b.sendLongMessage(detailedSummary, maxMessageLength)
}

// StartMessageHandler 启动消息处理器
func (b *Bot) StartMessageHandler() {
	log.Println("Starting Telegram message handler...")

	// 获取更新通道
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	// 启动消息处理协程
	go b.processMessages(updates)
}

// StopMessageHandler 停止消息处理器
func (b *Bot) StopMessageHandler() {
	log.Println("Stopping Telegram message handler...")
	close(b.stopHandler)
}

// processMessages 处理消息
func (b *Bot) processMessages(updates tgbotapi.UpdatesChannel) {
	for {
		select {
		case update := <-updates:
			if update.Message == nil {
				continue
			}

			// 只处理指定chatID的消息
			if update.Message.Chat.ID != b.chatID {
				continue
			}

			// 处理用户消息
			go b.HandleUserMessage(update)

		case <-b.stopHandler:
			return
		}
	}
}

// HandleUserMessage 处理用户消息
func (b *Bot) HandleUserMessage(update tgbotapi.Update) {
	message := strings.TrimSpace(update.Message.Text)
	log.Printf("Received message: %s", message)

	// 处理 resend 命令
	if strings.ToLower(message) == "resend" {
		b.handleResendRequest(update)
		return
	}

	// 尝试解析为纯数字
	if storyNumber, err := strconv.Atoi(message); err == nil {
		// 用户发送了纯数字编号
		b.handleStoryRequest(update, storyNumber, message)
		return
	}

	// 用户发送了非数字消息，发送帮助信息
	helpMessage := `🤖 Hacker News 每日总结机器人

💡 使用方法：
- 回复故事编号获取详细总结，例如：1、2、3
- 发送 "resend" 重新获取过去24小时的热点总结
- 每日18:00会自动推送当日热门故事总结

📝 当前支持的操作：
- 查看当日故事详细总结
- 重新获取过去24小时热点总结
- 自动接收每日热点推送

如有问题请联系管理员。`
	b.sendReply(update.Message, helpMessage)
}

// handleStoryRequest 处理故事详细总结请求
func (b *Bot) handleStoryRequest(update tgbotapi.Update, storyNumber int, _ string) {
	// 立即发送正在处理的提示信息
	processingMsg := fmt.Sprintf("🔄 正在为您生成故事 [%d] 的详细总结，请稍候...", storyNumber)
	if err := b.sendReply(update.Message, processingMsg); err != nil {
		log.Printf("Failed to send processing message: %v", err)
		return
	}

	// 获取今天的日期
	today := time.Now().Format("2006-01-02")

	// 发送详细总结
	if err := b.SendDetailedSummary(storyNumber, today); err != nil {
		log.Printf("Failed to send detailed summary: %v", err)
		// 发送错误信息
		errorMsg := fmt.Sprintf("❌ 获取故事 [%d] 的详细总结失败: %v", storyNumber, err)
		b.sendReply(update.Message, errorMsg)
		return
	}

	// 发送完成确认消息
	completionMsg := fmt.Sprintf("✅ 故事 [%d] 的详细总结已发送完成！", storyNumber)
	b.sendReply(update.Message, completionMsg)
}

// handleResendRequest 处理重新发送请求
func (b *Bot) handleResendRequest(update tgbotapi.Update) {
	// 立即发送正在处理的提示信息
	processingMsg := "🔄 正在重新获取过去24小时的热点总结，请稍候..."
	if err := b.sendReply(update.Message, processingMsg); err != nil {
		log.Printf("Failed to send processing message: %v", err)
		return
	}

	// 获取今天的日期
	today := time.Now().Format("2006-01-02")

	// 执行重新发送流程
	if err := b.ResendDailySummary(today); err != nil {
		log.Printf("Failed to resend daily summary: %v", err)
		// 发送错误信息
		errorMsg := fmt.Sprintf("❌ 重新获取热点总结失败: %v", err)
		b.sendReply(update.Message, errorMsg)
		return
	}

	// 发送完成确认消息
	completionMsg := "✅ 过去24小时的热点总结已重新发送完成！"
	b.sendReply(update.Message, completionMsg)
}

// ProcessDailySummary 处理每日总结的核心逻辑
func (b *Bot) ProcessDailySummary(date string, maxStories int) error {
	// 检查客户端是否已设置
	if b.aiClient == nil || b.hnClient == nil {
		return fmt.Errorf("AI或Hacker News客户端未初始化")
	}

	// 1. 获取热门故事
	log.Println("Fetching top stories")

	stories, err := b.hnClient.GetTopStoriesByDate(date, maxStories)
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

		content, err := b.hnClient.GetStoryContent(story)
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
	dailySummaryWithNumbers, err := b.aiClient.SummarizeStoriesWithNumbers(storyContents, stories, date)
	if err != nil {
		return fmt.Errorf("failed to summarize stories with numbers: %w", err)
	}

	// 4. 发送到 Telegram (带编号)
	log.Println("Sending numbered summary to Telegram...")
	if err := b.SendDailySummaryWithNumbers(dailySummaryWithNumbers); err != nil {
		return fmt.Errorf("failed to send numbered summary to telegram: %w", err)
	}

	log.Println("Successfully processed and sent numbered daily summary")
	return nil
}

// ResendDailySummary 重新发送每日总结
func (b *Bot) ResendDailySummary(date string) error {
	// 使用配置的最大故事数量
	return b.ProcessDailySummary(date, b.maxStories)
}

// sendReply 回复消息
func (b *Bot) sendReply(message *tgbotapi.Message, text string) error {
	reply := tgbotapi.NewMessage(message.Chat.ID, text)
	reply.ReplyToMessageID = message.MessageID
	reply.ParseMode = tgbotapi.ModeMarkdown
	reply.DisableWebPagePreview = true

	_, err := b.api.Send(reply)
	if err != nil {
		return fmt.Errorf("failed to send reply: %w", err)
	}

	return nil
}
