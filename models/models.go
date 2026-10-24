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

// QualityResult — 검색 품질 측정 결과
type QualityResult struct {
	Topic    string
	Version  string
	Note     string
	AvgScore float64
	MaxScore float64
	MinScore float64
	RunAt    string
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

type ResearchPlan struct {
	Steps    []PlanStep `json:"steps"`    // Tool 실행 순서
	Goal     PlanGoal   `json:"goal"`     // 달성 기준
}

type PlanStep struct {
	Tool    string   `json:"tool"`    // tavily, exa, scholar
	Queries []string `json:"queries"` // 검색 쿼리 목록
	Purpose string   `json:"purpose"` // 이 Tool을 쓰는 이유
}

type PlanGoal struct {
	MinSources  int     `json:"min_sources"`   // 최소 출처 수
	MinScore    float64 `json:"min_score"`     // 최소 품질 점수
	NeedStats   bool    `json:"need_stats"`    // 수치 필요 여부
	NeedPapers  bool    `json:"need_papers"`   // 논문 필요 여부
	NeedRecent  bool    `json:"need_recent"`   // 최신 자료 필요 여부
}

// EvalResult — 결과 평가
type EvalResult struct {
	SourceCount  int
	AvgScore     float64
	HasStats     bool
	HasPapers    bool
	RecentCount  int
	GoalMet      bool
	Missing      []string // 부족한 것 목록
}

// AgentAction — 다음 행동
type AgentAction struct {
	Action  string   `json:"action"`
	Queries []string `json:"queries"`
	Reason  string   `json:"reason"`
}