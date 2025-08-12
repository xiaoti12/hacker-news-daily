package ai

import (
	"fmt"
	"strings"

	"github.com/go-resty/resty/v2"
)

type Client struct {
	httpClient *resty.Client
	baseURL    string
	apiKey     string
	model      string
	maxTokens  int
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model     string        `json:"model"`
	Messages  []ChatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens,omitempty"`
}

type ChatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

func NewClient(baseURL, apiKey, model string, maxTokens int) *Client {
	client := resty.New().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", "Bearer "+apiKey)

	return &Client{
		httpClient: client,
		baseURL:    baseURL,
		apiKey:     apiKey,
		model:      model,
		maxTokens:  maxTokens,
	}
}

// SummarizeStories 总结多个故事
func (c *Client) SummarizeStories(stories []string, date string) (string, error) {
	systemPrompt := `你是 Hacker News 中文播客的编辑，擅长将技术文章和讨论整理成引人入胜的内容。

工作目标：
- 为每个 Hacker News 故事单独生成一个完整的段落总结
- 每个段落应包含：故事标题、核心内容概述、关键技术要点、评论区的主要观点和讨论亮点
- 总结故事的核心观点和技术价值
- 整理评论区的不同观点、争议点和深度见解
- 用简洁明了的中文呈现，专业术语可保留英文

输出要求：
- 每个故事生成一个独立的段落，段落之间用空行分隔
- 每个段落以故事标题开头，格式为：**标题名称**
- 段落内容应该是连贯的文字描述，不使用列表或子标题
- 内容要有洞察力和可读性，适合技术从业者快速了解
- 避免政治敏感内容
- 重点关注技术趋势、产品发布、行业动态、开发者讨论等
- 每个段落长度控制在150-300字之间`

	userPrompt := fmt.Sprintf("请为以下 %s 的 Hacker News 热门故事分别生成独立的段落总结。每个故事应该生成一个完整的段落，包含标题、内容要点和评论精华：\n\n%s",
		date, strings.Join(stories, "\n\n---\n\n"))

	request := ChatRequest{
		Model: c.model,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens: c.maxTokens,
	}

	var response ChatResponse
	resp, err := c.httpClient.R().
		SetBody(request).
		SetResult(&response).
		Post(c.baseURL + "/chat/completions")

	if err != nil {
		return "", fmt.Errorf("failed to call AI API: %w", err)
	}

	if resp.StatusCode() != 200 {
		return "", fmt.Errorf("AI API returned status code: %d, body: %s", resp.StatusCode(), resp.String())
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response from AI")
	}

	return response.Choices[0].Message.Content, nil
}

// CreateDailySummary 创建每日总结
func (c *Client) CreateDailySummary(storySummaries string, date string) (string, error) {
	systemPrompt := `你是 Hacker News 每日总结的编辑，负责将已经按故事分段的内容整合为一份完整的每日报告。

工作目标：
- 保持每个故事段落的完整性和独立性
- 在开头添加简短的日期介绍和当日要点概述
- 保持原有的故事段落结构，确保每个故事都有完整的总结
- 在结尾添加简短的总结和技术趋势展望
- 为技术从业者提供易于阅读的信息摘要

输出要求：
- 以日期开头，用1-2句话概述当日的主要技术热点
- 保持输入中每个故事段落的原始格式和内容
- 每个故事段落之间保持空行分隔
- 在所有故事段落后，添加一个简短的"今日总结"段落
- 使用连贯的中文表达，避免使用markdown格式标记
- 语言简洁明了，富有洞察力
- 总结段落应该提炼当日的主要技术趋势和值得关注的发展方向`

	userPrompt := fmt.Sprintf("请将以下 %s 的故事段落总结整合为一份完整的每日报告。请保持每个故事段落的完整性，并在开头和结尾添加适当的介绍和总结：\n\n%s", date, storySummaries)

	request := ChatRequest{
		Model: c.model,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens: c.maxTokens,
	}

	var response ChatResponse
	resp, err := c.httpClient.R().
		SetBody(request).
		SetResult(&response).
		Post(c.baseURL + "/chat/completions")

	if err != nil {
		return "", fmt.Errorf("failed to call AI API: %w", err)
	}

	if resp.StatusCode() != 200 {
		return "", fmt.Errorf("AI API returned status code: %d, body: %s", resp.StatusCode(), resp.String())
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response from AI")
	}

	return response.Choices[0].Message.Content, nil
}
