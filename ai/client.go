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
- 分析每个 Hacker News 故事的主要内容和讨论重点
- 总结故事的核心观点和技术要点
- 整理评论区的不同观点和深度讨论
- 用简洁明了的中文呈现，专业术语可保留英文

输出要求：
- 使用 Markdown 格式
- 每个故事用二级标题分隔
- 内容要有洞察力，适合技术从业者阅读
- 避免政治敏感内容
- 重点关注技术趋势、产品发布、行业动态等`

	userPrompt := fmt.Sprintf("请总结以下 %s 的 Hacker News 热门故事：\n\n%s",
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
	systemPrompt := `你是 Hacker News 每日总结的编辑，负责将多个技术故事整合为一份每日报告。

工作目标：
- 将多个故事总结整合为一份连贯的每日报告
- 突出当日的重要技术趋势和热点话题
- 用引人入胜的方式呈现技术新闻
- 为技术从业者提供有价值的信息摘要

输出要求：
- 使用 Markdown 格式
- 以日期开头，简要介绍当日要点
- 按重要性和相关性组织内容
- 语言简洁明了，富有洞察力
- 在结尾提供简短的总结和展望`

	userPrompt := fmt.Sprintf("请将以下 %s 的故事总结整合为一份每日报告：\n\n%s", date, storySummaries)

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
