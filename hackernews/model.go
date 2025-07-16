package hackernews

type Story struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	URL           string `json:"url"`
	Score         int    `json:"score"`
	By            string `json:"by"`
	Time          int64  `json:"time"`
	Type          string `json:"type"`
	Text          string `json:"text"`
	Kids          []int  `json:"kids"`
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
