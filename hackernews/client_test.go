package hackernews

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetTopStoriesByDate(t *testing.T) {
	client := NewClient(30)

	stories, err := client.GetTopStoriesByDate(time.Now().Format("2006-01-02"), 5)
	assert.NoError(t, err)
	assert.NotEmpty(t, stories)

	if len(stories) > 0 {
		story := &stories[0]
		storyStr, _ := json.MarshalIndent(story, "", "  ")
		t.Logf("%s", storyStr)
	}
}

// BenchmarkGetCommentsParallel 测试并发获取评论的性能
func BenchmarkGetCommentsParallel(b *testing.B) {
	client := NewClient(30)

	// 使用一个有很多评论的故事ID进行测试
	storyID := 38905019

	// 先获取故事信息以获得评论ID列表
	story, _, err := client.GetStoryWithComments(storyID)
	if err != nil {
		b.Fatalf("Failed to get story: %v", err)
	}

	if len(story.Kids) == 0 {
		b.Skip("Story has no comments")
	}

	// 限制评论数量用于基准测试
	maxComments := 10
	if len(story.Kids) > maxComments {
		story.Kids = story.Kids[:maxComments]
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		comments := client.getCommentsParallel(story.Kids, 2)
		if len(comments) == 0 {
			b.Errorf("Expected comments but got none")
		}
	}
}

// TestGetCommentsParallelVsSequential 比较并发和串行获取评论的性能
func TestGetCommentsParallelVsSequential(t *testing.T) {
	client := NewClient(30)

	// 使用一个有评论的故事ID
	storyID := 38905019

	// 获取故事信息
	story, _, err := client.GetStoryWithComments(storyID)
	if err != nil {
		t.Fatalf("Failed to get story: %v", err)
	}

	if len(story.Kids) == 0 {
		t.Skip("Story has no comments")
	}

	// 限制评论数量用于测试
	maxComments := 5
	if len(story.Kids) > maxComments {
		story.Kids = story.Kids[:maxComments]
	}

	// 测试并发获取
	start := time.Now()
	parallelComments := client.getCommentsParallel(story.Kids, 1)
	parallelDuration := time.Since(start)

	// 测试串行获取（模拟原来的方式）
	start = time.Now()
	var sequentialComments []Comment
	for _, kidID := range story.Kids {
		if comment, err := client.getComment(kidID, 1); err == nil && comment != nil {
			sequentialComments = append(sequentialComments, *comment)
		}
	}
	sequentialDuration := time.Since(start)

	t.Logf("并发获取 %d 条评论耗时: %v", len(parallelComments), parallelDuration)
	t.Logf("串行获取 %d 条评论耗时: %v", len(sequentialComments), sequentialDuration)

	// 验证结果数量相近（可能因为网络问题略有差异）
	assert.True(t, len(parallelComments) > 0, "并发获取应该返回评论")
	assert.True(t, len(sequentialComments) > 0, "串行获取应该返回评论")

	// 在理想情况下，并发应该更快
	if parallelDuration < sequentialDuration {
		t.Logf("并发获取比串行获取快 %v", sequentialDuration-parallelDuration)
	} else {
		t.Logf("在这次测试中串行获取更快，可能由于网络延迟或评论数量较少")
	}
}

// TestGetStoryContent 测试GetStoryContent函数
func TestGetStoryContent(t *testing.T) {
	client := NewClient(30)

	tests := []struct {
		name     string
		story    Story
		expected []string // 期望包含的内容片段
	}{
		{
			name: "基本故事信息",
			story: Story{
				ID:            12345,
				Title:         "测试标题",
				URL:           "https://example.com",
				HackerNewsURL: "https://news.ycombinator.com/item?id=12345",
				Score:         100,
				By:            "testuser",
				Text:          "",
				Kids:          []int{}, // 无评论
			},
			expected: []string{
				"标题: 测试标题",
				"链接: https://example.com",
				"HN链接: https://news.ycombinator.com/item?id=12345",
				"分数: 100 | 作者: testuser",
			},
		},
		{
			name: "包含正文的故事",
			story: Story{
				ID:            12346,
				Title:         "有正文的故事",
				URL:           "https://example.com/story",
				HackerNewsURL: "https://news.ycombinator.com/item?id=12346",
				Score:         200,
				By:            "author",
				Text:          "<p>这是一段<strong>HTML</strong>正文内容&amp;测试</p>",
				Kids:          []int{},
			},
			expected: []string{
				"标题: 有正文的故事",
				"正文内容:",
				"这是一段HTML正文内容&测试", // HTML标签应该被清理
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := client.GetStoryContent(tt.story)
			assert.NoError(t, err)
			assert.NotEmpty(t, content)

			// 检查是否包含期望的内容片段
			for _, expected := range tt.expected {
				assert.Contains(t, content, expected, "内容应该包含: %s", expected)
			}

			t.Logf("生成的内容:\n%s", content)
		})
	}
}

// TestGetStoryContentWithComments 测试包含评论的故事内容生成
func TestGetStoryContentWithComments(t *testing.T) {
	client := NewClient(30)

	// 使用一个真实的故事ID进行集成测试
	story := Story{
		ID:            38905019, // 使用现有测试中的故事ID
		Title:         "集成测试故事",
		URL:           "https://example.com",
		HackerNewsURL: "https://news.ycombinator.com/item?id=38905019",
		Score:         150,
		By:            "testauthor",
		Text:          "这是故事正文",
	}

	content, err := client.GetStoryContent(story)
	assert.NoError(t, err)
	assert.NotEmpty(t, content)

	// 检查基本信息是否存在
	assert.Contains(t, content, "标题: 集成测试故事")
	assert.Contains(t, content, "分数: 150 | 作者: testauthor")
	assert.Contains(t, content, "正文内容:")
	assert.Contains(t, content, "这是故事正文")

	t.Logf("包含评论的内容长度: %d", len(content))
}

// TestCleanHTMLText 测试HTML清理函数
func TestCleanHTMLText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "基本HTML标签清理",
			input:    "<p>Hello <strong>world</strong>!</p>",
			expected: "Hello world!",
		},
		{
			name:     "HTML实体解码",
			input:    "Test &lt;code&gt; &amp; &quot;quotes&quot; &#x27;apostrophe&#x27;",
			expected: "Test <code> & \"quotes\" 'apostrophe'",
		},
		{
			name:     "复杂HTML内容",
			input:    "<div><p>段落1</p><br/><p>段落2 with <a href=\"#\">链接</a></p></div>",
			expected: "段落1段落2 with 链接",
		},
		{
			name:     "空字符串",
			input:    "",
			expected: "",
		},
		{
			name:     "纯文本",
			input:    "纯文本内容",
			expected: "纯文本内容",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanHTMLText(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGetStoryContentErrorHandling 测试错误处理
func TestGetStoryContentErrorHandling(t *testing.T) {
	client := NewClient(1) // 设置很短的超时时间

	// 测试无效故事ID的情况
	story := Story{
		ID:            -1, // 无效ID
		Title:         "无效故事",
		URL:           "https://example.com",
		HackerNewsURL: "https://news.ycombinator.com/item?id=-1",
		Score:         0,
		By:            "nobody",
		Text:          "测试正文",
	}

	content, err := client.GetStoryContent(story)
	// 即使获取评论失败，函数也应该返回基本内容
	assert.NoError(t, err)
	assert.NotEmpty(t, content)
	assert.Contains(t, content, "标题: 无效故事")
	assert.Contains(t, content, "测试正文")
}
