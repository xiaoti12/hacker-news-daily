package hackernews

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

type Client struct {
	httpClient *resty.Client
	timeout    time.Duration
}

func NewClient(timeout int) *Client {
	client := resty.New().
		SetTimeout(time.Duration(timeout)*time.Second).
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")

	return &Client{
		httpClient: client,
		timeout:    time.Duration(timeout) * time.Second,
	}
}

// GetTopStoriesByDate 获取指定日期的热门故事
func (c *Client) GetTopStoriesByDate(date string, maxStories int) ([]Story, error) {
	// 如果传入空字符串，则获取过去24小时的内容
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	endTime := getTimeForDate(date)
	startTime := endTime.Add(-24 * time.Hour)
	if startTime.IsZero() || endTime.IsZero() {
		return nil, fmt.Errorf("invalid date format: %s", date)
	}
	return c.getTopStoriesByTime(startTime, endTime, maxStories)
}

func (c *Client) getTopStoriesByTime(startTime, endTime time.Time, maxStories int) ([]Story, error) {
	// 使用 HN 的搜索 API 获取指定时间段的热门故事
	url := "https://hn.algolia.com/api/v1/search_by_date"

	var response TopStoriesResponse

	resp, err := c.httpClient.R().
		SetResult(&response).
		SetQueryParams(map[string]string{
			"tags":           "front_page",
			"numericFilters": fmt.Sprintf("created_at_i>%d,created_at_i<%d", startTime.Unix(), endTime.Unix()),
			"hitsPerPage":    fmt.Sprintf("%d", maxStories),
		}).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch top stories: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode())
	}

	stories := make([]Story, 0, len(response.Hits))
	for _, hit := range response.Hits {
		story := Story{
			ID:            parseInt(hit.ObjectID),
			Title:         hit.Title,
			URL:           hit.URL,
			Score:         hit.Points,
			By:            hit.Author,
			Time:          hit.CreatedAtI,
			Text:          hit.StoryText,
			HackerNewsURL: fmt.Sprintf("https://news.ycombinator.com/item?id=%s", hit.ObjectID),
		}
		stories = append(stories, story)
	}

	return stories, nil
}

// GetStoryWithComments 获取故事详情和评论
func (c *Client) GetStoryWithComments(storyID int) (*Story, []Comment, error) {
	// 获取故事详情
	storyURL := fmt.Sprintf("https://hacker-news.firebaseio.com/v0/item/%d.json", storyID)

	var story Story
	resp, err := c.httpClient.R().
		SetResult(&story).
		Get(storyURL)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch story: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, nil, fmt.Errorf("story API returned status code: %d", resp.StatusCode())
	}

	// 获取评论
	comments := make([]Comment, 0)
	if len(story.Kids) > 0 {
		// 限制评论数量，避免请求过多
		maxComments := 20
		if len(story.Kids) > maxComments {
			story.Kids = story.Kids[:maxComments]
		}

		// 使用并发获取顶级评论
		comments = c.getCommentsParallel(story.Kids, 2)
	}

	return &story, comments, nil
}

// getComment 递归获取评论和子评论
func (c *Client) getComment(commentID int, maxDepth int) (*Comment, error) {
	if maxDepth <= 0 {
		return nil, nil
	}

	commentURL := fmt.Sprintf("https://hacker-news.firebaseio.com/v0/item/%d.json", commentID)

	var comment Comment
	resp, err := c.httpClient.R().
		SetResult(&comment).
		Get(commentURL)

	if err != nil {
		log.Printf("Failed to fetch comment %d: %v", commentID, err)
		return nil, err
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("comment API returned status code: %d", resp.StatusCode())
	}

	// 如果评论被删除或为空，跳过
	if comment.Text == "" || comment.Type != "comment" {
		return nil, nil
	}

	// TODO 子评论数设为配置项
	// 获取子评论（限制数量）
	if len(comment.Kids) > 0 && maxDepth > 1 {
		maxChildren := 5 // 限制子评论数量
		if len(comment.Kids) > maxChildren {
			comment.Kids = comment.Kids[:maxChildren]
		}

		// 使用并发获取子评论
		comment.Children = c.getCommentsParallel(comment.Kids, maxDepth-1)
	}

	return &comment, nil
}

// getCommentsParallel 并发获取多个评论
func (c *Client) getCommentsParallel(commentIDs []int, maxDepth int) []Comment {
	if len(commentIDs) == 0 {
		return nil
	}

	// 使用 channel 收集结果
	commentChan := make(chan Comment, len(commentIDs))
	var wg sync.WaitGroup

	// 启动 goroutine 并发获取评论
	for _, commentID := range commentIDs {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if comment, err := c.getComment(id, maxDepth); err == nil && comment != nil {
				commentChan <- *comment
			}
		}(commentID)
	}

	// 等待所有 goroutine 完成
	go func() {
		wg.Wait()
		close(commentChan)
	}()

	// 收集结果
	var comments []Comment
	for comment := range commentChan {
		comments = append(comments, comment)
	}

	return comments
}

// GetStoryContent 获取故事完整内容（包括正文和评论）
func (c *Client) GetStoryContent(story Story) (string, error) {
	var content strings.Builder

	// 添加标题和基本信息
	content.WriteString(fmt.Sprintf("标题: %s\n", story.Title))
	content.WriteString(fmt.Sprintf("链接: %s\n", story.URL))
	content.WriteString(fmt.Sprintf("HN链接: %s\n", story.HackerNewsURL))
	content.WriteString(fmt.Sprintf("分数: %d | 作者: %s\n\n", story.Score, story.By))

	// 如果有正文内容，添加正文
	if story.Text != "" {
		content.WriteString("正文内容:\n")
		content.WriteString(cleanHTMLText(story.Text))
		content.WriteString("\n\n")
	}

	// 获取评论
	_, comments, err := c.GetStoryWithComments(story.ID)
	if err != nil {
		log.Printf("Failed to get comments for story %d: %v", story.ID, err)
	} else if len(comments) > 0 {
		content.WriteString("热门评论:\n")
		for i, comment := range comments {
			if i >= 10 { // 限制评论数量
				break
			}
			content.WriteString(fmt.Sprintf("\n评论 %d (作者: %s):\n", i+1, comment.By))
			content.WriteString(cleanHTMLText(comment.Text))

			// 添加子评论
			if len(comment.Children) > 0 {
				for j, child := range comment.Children {
					if j >= 3 { // 限制子评论数量
						break
					}
					content.WriteString(fmt.Sprintf("\n  └─ 回复 (作者: %s): %s", child.By, cleanHTMLText(child.Text)))
				}
			}
			content.WriteString("\n")
		}
	}

	return content.String(), nil
}

// 辅助函数

func getTimeForDate(date string) time.Time {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return time.Now()
	}
	return t
}

func parseInt(s string) int {
	var result int
	fmt.Sscanf(s, "%d", &result)
	return result
}

func cleanHTMLText(htmlText string) string {
	// 简单的HTML标签清理
	re := regexp.MustCompile(`<[^>]*>`)
	cleaned := re.ReplaceAllString(htmlText, "")

	// 解码HTML实体
	cleaned = strings.ReplaceAll(cleaned, "&lt;", "<")
	cleaned = strings.ReplaceAll(cleaned, "&gt;", ">")
	cleaned = strings.ReplaceAll(cleaned, "&amp;", "&")
	cleaned = strings.ReplaceAll(cleaned, "&quot;", "\"")
	cleaned = strings.ReplaceAll(cleaned, "&#x27;", "'")

	return strings.TrimSpace(cleaned)
}
