package models

// Report — 최종 결과물
type Report struct {
	Topic    string
	Markdown string
	Cards    []CardContent
	Sources  []string
}

// CardContent — 카드뉴스 한 장
type CardContent struct {
	CardNumber int
	Headline   string
	Body       string
	Emoji      string
	Tag        string
}

// BenchmarkResult — 벤치마크 결과
type BenchmarkResult struct {
	Topic       string
	SeqTime     int64
	ParTime     int64
	Workers     int
	Improvement float64
	Reduction   float64
	RunAt       string
}