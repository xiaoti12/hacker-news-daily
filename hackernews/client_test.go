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
