package hackernews

import (
	"fmt"
	"log"
	"regexp"
	"strings"
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
		SetHeader("User-Agent", "HackerNews-Daily-Bot/1.0")

	return &Client{
		httpClient: client,
		timeout:    time.Duration(timeout) * time.Second,
	}
}

// GetTopStoriesByDate 获取指定日期的热门故事
func (c *Client) GetTopStoriesByDate(date string, maxStories int) ([]Story, error) {
	// 使用 HN 的搜索 API 获取指定日期的热门故事
	url := fmt.Sprintf("https://hn.algolia.com/api/v1/search_by_date?tags=front_page&numericFilters=created_at_i>%d,created_at_i<%d&hitsPerPage=%d",
		getTimestampForDate(date), getTimestampForDate(date)+86400, maxStories)

	var response struct {
		Hits []struct {
			ObjectID    string `json:"objectID"`
			Title       string `json:"title"`
			URL         string `json:"url"`
			Points      int    `json:"points"`
			Author      string `json:"author"`
			CreatedAtI  int64  `json:"created_at_i"`
			StoryText   string `json:"story_text"`
			NumComments int    `json:"num_comments"`
		} `json:"hits"`
	}

	resp, err := c.httpClient.R().
		SetResult(&response).
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

		for _, kidID := range story.Kids {
			if comment, err := c.getComment(kidID, 2); err == nil && comment != nil {
				comments = append(comments, *comment)
			}
		}
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

	// 获取子评论（限制数量）
	if len(comment.Kids) > 0 && maxDepth > 1 {
		maxChildren := 5 // 限制子评论数量
		if len(comment.Kids) > maxChildren {
			comment.Kids = comment.Kids[:maxChildren]
		}

		for _, kidID := range comment.Kids {
			if child, err := c.getComment(kidID, maxDepth-1); err == nil && child != nil {
				comment.Children = append(comment.Children, *child)
			}
		}
	}

	return &comment, nil
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

func getTimestampForDate(date string) int64 {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return time.Now().Unix()
	}
	return t.Unix()
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
