package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSummarizeStoriesInputValidation 测试SummarizeStories输入验证
func TestSummarizeStoriesInputValidation(t *testing.T) {
	// 创建客户端但不进行实际API调用
	client := NewClient("", "", "gpt-4o", 2000)

	tests := []struct {
		name    string
		stories []string
		date    string
		skip    bool
	}{
		{
			name:    "空故事列表",
			stories: []string{},
			date:    "2024-01-15",
			skip:    true, // 跳过实际执行
		},
		{
			name:    "nil故事列表",
			stories: nil,
			date:    "2024-01-15",
			skip:    true,
		},
		{
			name: "正常输入",
			stories: []string{
				"标题: Go 1.21 发布\n链接: https://golang.org/doc/go1.21\n分数: 500 | 作者: golang",
				"标题: 新的AI框架发布\n链接: https://example.com/ai\n分数: 300 | 作者: aidev",
			},
			date: "2024-01-15",
			skip: false,
		},
		{
			name:    "空日期",
			stories: []string{"测试故事"},
			date:    "",
			skip:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip("跳过实际API调用测试")
			}

			// 这里可以添加实际的测试逻辑，但目前跳过
			data, err := client.SummarizeStories(tt.stories, tt.date)

			assert.NoError(t, err, "预期正常输入不会出错")
			t.Logf("生成的总结内容:\n%s", data)

		})
	}
}

// TestCreateDailySummaryInputValidation 测试CreateDailySummary输入验证
func TestCreateDailySummaryInputValidation(t *testing.T) {
	// 创建客户端但不进行实际API调用
	client := NewClient("", "", "gpt-4o", 2000)

	tests := []struct {
		name           string
		storySummaries string
		date           string
		skip           bool
	}{
		{
			name:           "空总结内容",
			storySummaries: "",
			date:           "2024-01-15",
			skip:           true,
		},
		{
			name: "正常输入",
			storySummaries: `## Go 1.21 发布

Go语言发布了1.21版本，带来了多项重要更新...

## 新的AI框架发布

一个新的AI开发框架正式发布...`,
			date: "2024-01-15",
			skip: true,
		},
		{
			name:           "空日期",
			storySummaries: "测试总结内容",
			date:           "",
			skip:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip("跳过实际API调用测试")
			}

			// 这里可以添加实际的测试逻辑，但目前跳过
			data, err := client.CreateDailySummary(tt.storySummaries, tt.date)

			assert.NoError(t, err, "预期正常输入不会出错")
			t.Logf("生成的每日总结内容:\n%s", data)
		})
	}
}
