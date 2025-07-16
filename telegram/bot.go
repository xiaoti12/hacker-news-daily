package telegram

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api    *tgbotapi.BotAPI
	chatID int64
}

func NewBot(token, chatIDStr string) (*Bot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %w", err)
	}

	log.Printf("Telegram bot authorized on account %s", bot.Self.UserName)

	return &Bot{
		api:    bot,
		chatID: chatID,
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
