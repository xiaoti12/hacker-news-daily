package ai

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-resty/resty/v2"
	"hacker-news-daily/hackernews"
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
- 为技术从业者提供易于阅读的信息摘要

输出要求：
- 以日期开头，用1-2句话概述当日的主要技术热点
- 保持输入中每个故事段落的原始格式和内容
- 每个故事段落之间保持空行分隔
- 使用连贯的中文表达，避免使用markdown格式标记
- 语言简洁明了，富有洞察力`

	userPrompt := fmt.Sprintf("请将以下 %s 的故事段落总结整合为一份完整的每日报告。请保持每个故事段落的完整性，并在开头添加适当的介绍：\n\n%s", date, storySummaries)

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

// SummarizeStoriesWithNumbers 生成带编号的故事总结
func (c *Client) SummarizeStoriesWithNumbers(stories []string, storiesInfo []hackernews.Story, date string) (*hackernews.DailySummaryWithNumbers, error) {
	systemPrompt := `你是 Hacker News 中文播客的编辑，擅长将技术文章和讨论整理成引人入胜的内容。

工作目标：
- 为每个 Hacker News 故事单独生成一个完整的段落总结
- 每个段落应包含：故事标题、核心内容概述、关键技术要点、评论区的主要观点和讨论亮点
- 总结故事的核心观点和技术价值
- 整理评论区的不同观点、争议点和深度见解
- 用简洁明了的中文呈现，专业术语可保留英文

输出要求：
- 每个故事生成一个独立的段落，段落之间用空行分隔
- 每个段落以故事编号开头，格式为：[编号] **标题名称**
- 段落内容应该是连贯的文字描述，不使用列表或子标题
- 内容要有洞察力和可读性，适合技术从业者快速了解
- 避免政治敏感内容
- 重点关注技术趋势、产品发布、行业动态、开发者讨论等
- 每个段落长度控制在150-300字之间`

	// 构建包含故事信息的prompt
	var storiesWithInfo []string
	for i, story := range stories {
		storyInfo := fmt.Sprintf("故事 %d:\n标题: %s\nURL: %s\n分数: %d\n作者: %s\n内容:\n%s", 
			i+1, storiesInfo[i].Title, storiesInfo[i].URL, storiesInfo[i].Score, storiesInfo[i].By, story)
		storiesWithInfo = append(storiesWithInfo, storyInfo)
	}

	userPrompt := fmt.Sprintf("请为以下 %s 的 Hacker News 热门故事分别生成带编号的段落总结。每个故事应该生成一个完整的段落，包含编号、标题、内容要点和评论精华：\n\n%s",
		date, strings.Join(storiesWithInfo, "\n\n---\n\n"))

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
		return nil, fmt.Errorf("failed to call AI API: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("AI API returned status code: %d, body: %s", resp.StatusCode(), resp.String())
	}

	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no response from AI")
	}

	// 解析AI返回的带编号总结
	summaryText := response.Choices[0].Message.Content
	storySummaries := c.parseNumberedSummaries(summaryText, storiesInfo)

	return &hackernews.DailySummaryWithNumbers{
		Date:           date,
		Stories:        storiesInfo,
		StorySummaries: storySummaries,
	}, nil
}

// GenerateDetailedSummary 生成单个故事的详细总结
func (c *Client) GenerateDetailedSummary(story hackernews.Story, content string) (string, error) {
	systemPrompt := `你是 Hacker News 深度分析专家，擅长对技术故事进行深入剖析和详细总结。

工作目标：
- 对单个 Hacker News 故事进行全面、深入的分析
- 提取技术细节、实现方法、架构设计等核心信息
- 分析评论区的技术讨论、观点碰撞、争议焦点
- 评估该故事的技术价值和行业影响
- 为技术从业者提供深度洞察

输出要求：
- 生成结构化的详细总结，包含以下部分：
  1. **核心概述**：故事背景和主要内容
  2. **技术要点**：关键技术细节、实现方法、架构设计
  3. **讨论精华**：评论区的重要观点、技术争论、深度见解
  4. **价值分析**：技术价值、行业影响、学习要点
- 使用清晰的段落分隔，每部分都有明确的标题
- 内容详实、专业，适合深度技术阅读
- 保持客观中立的技术视角
- 总结长度控制在800-1200字之间`

	userPrompt := fmt.Sprintf(`请对以下 Hacker News 故事进行深度分析和详细总结：

标题: %s
URL: %s
分数: %d
作者: %s

故事内容:
%s

请按照要求的结构生成详细的技术分析总结。`, story.Title, story.URL, story.Score, story.By, content)

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

// parseNumberedSummaries 解析AI返回的带编号总结
func (c *Client) parseNumberedSummaries(summaryText string, stories []hackernews.Story) []hackernews.StoryWithNumber {
	lines := strings.Split(summaryText, "\n")
	var storySummaries []hackernews.StoryWithNumber
	
	var currentSummary strings.Builder
	var currentNumber int
	
	for _, line := range lines {
		// 检查是否是新的故事编号行
		if matches := c.isNumberedStoryLine(line); matches != nil {
			// 保存前一个故事的总结
			if currentNumber > 0 && currentSummary.Len() > 0 {
				if currentNumber-1 < len(stories) {
					storySummaries = append(storySummaries, hackernews.StoryWithNumber{
						Number:  currentNumber,
						StoryID: stories[currentNumber-1].ID,
						Title:   stories[currentNumber-1].Title,
						Summary: strings.TrimSpace(currentSummary.String()),
						Content: "", // 这里可以在调用时填充
					})
				}
			}
			
			// 开始新故事
			currentNumber = matches[0]
			currentSummary.Reset()
			// 添加去除编号后的行内容
			cleanLine := c.cleanNumberedLine(line, currentNumber)
			currentSummary.WriteString(cleanLine)
		} else {
			if currentNumber > 0 {
				currentSummary.WriteString("\n")
				currentSummary.WriteString(line)
			}
		}
	}
	
	// 添加最后一个故事
	if currentNumber > 0 && currentSummary.Len() > 0 && currentNumber-1 < len(stories) {
		storySummaries = append(storySummaries, hackernews.StoryWithNumber{
			Number:  currentNumber,
			StoryID: stories[currentNumber-1].ID,
			Title:   stories[currentNumber-1].Title,
			Summary: strings.TrimSpace(currentSummary.String()),
			Content: "",
		})
	}
	
	return storySummaries
}

// isNumberedStoryLine 检查是否是带编号的故事行
func (c *Client) isNumberedStoryLine(line string) []int {
	// 匹配格式: [1] **标题** 或 [1] 标题
	if strings.HasPrefix(line, "[") {
		parts := strings.SplitN(line, "]", 2)
		if len(parts) == 2 {
			if num, err := strconv.Atoi(strings.Trim(parts[0], "[]")); err == nil {
				return []int{num}
			}
		}
	}
	return nil
}

// cleanNumberedLine 清理编号行的格式
func (c *Client) cleanNumberedLine(line string, number int) string {
	// 去除编号部分，保留标题和内容
	parts := strings.SplitN(line, "]", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1])
	}
	return line
}
