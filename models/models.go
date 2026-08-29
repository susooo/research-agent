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

// TradeoffResult — 워커 하나의 트레이드오프 측정 결과
type TradeoffResult struct {
	Workers       int
	ElapsedMs     int64
	MemoryMB  	  float64
	Failed        bool
}

// TradeoffBenchmarkResult — 트레이드오프 벤치마크 결과
type TradeoffBenchmarkResult struct {
	Topic   string
	RunAt   string
	Results []TradeoffResult
}