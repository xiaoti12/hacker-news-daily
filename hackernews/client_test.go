package hackernews

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClientIntegration(t *testing.T) {
	client := NewClient(30)

	stories, err := client.GetTopStoriesByDate(time.Now().Format("2006-01-02"), 5)
	assert.NoError(t, err)
	assert.NotEmpty(t, stories)

	if len(stories) > 0 {
		story := &stories[0]
		storyStr, _ := json.MarshalIndent(story, "", "  ")
		t.Logf("%s", storyStr)
	}

	story, comments, err := client.GetStoryWithComments(38905019)
	assert.NoError(t, err)
	assert.NotNil(t, story)
	assert.NotNil(t, comments)

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
