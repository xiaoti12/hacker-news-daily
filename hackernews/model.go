package hackernews

type Story struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	URL           string `json:"url"`
	Score         int    `json:"score"`
	By            string `json:"by"`
	Time          int64  `json:"time"`
	Text          string `json:"text"`
	Kids          []int  `json:"kids"` // 评论ID列表
	HackerNewsURL string `json:"hacker_news_url"`
}

type Comment struct {
	ID       int       `json:"id"`
	By       string    `json:"by"`
	Text     string    `json:"text"`
	Time     int64     `json:"time"`
	Kids     []int     `json:"kids"`
	Parent   int       `json:"parent"`
	Type     string    `json:"type"`
	Children []Comment `json:"children,omitempty"`
}

type DailyStories struct {
	Date    string  `json:"date"`
	Stories []Story `json:"stories"`
}

type TopStoriesResponse struct {
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
