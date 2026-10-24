package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"runtime"

	"research-agent/config"
	"research-agent/models"
	"research-agent/orchestrator"
	"research-agent/renderer"
	"research-agent/tools"
)

// Agent — 전체 흐름 지휘자
type Agent struct {
	cfg        *config.Config
	httpClient *http.Client
	orch       *orchestrator.Orchestrator
	exa        *tools.ExaSearcher
}

type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// New — Agent 생성
func New(cfg *config.Config) *Agent {
	searcher := tools.NewSearcher(cfg.TavilyAPIKey)
	orch := orchestrator.New(searcher, cfg.MaxWorkers)
	return &Agent{
		cfg: cfg,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		orch: orch,
		exa:        tools.NewExaSearcher(cfg.ExaAPIKey),
	}
}

// ────────────────────────────────────────
// Research — 전체 리서치 흐름
// ────────────────────────────────────────
func (a *Agent) Research(topic string) (*models.Report, error) {
	ctx := context.Background()
	totalStart := time.Now()
	const maxLoop = 3

	fmt.Println("\n🤖 Agent 리서치 시작")
	fmt.Printf("주제: %s\n", topic)

	// 1단계: Plan 수립
	fmt.Println("\n[1단계] 리서치 계획 수립 중...")
	step1Start := time.Now()
	researchPlan, err := a.plan(ctx, topic)
	step1Time := time.Since(step1Start)
	if err != nil {
		return nil, fmt.Errorf("계획 수립 실패: %w", err)
	}

	// Plan 출력
	for i, step := range researchPlan.Steps {
		fmt.Printf("  %d. [%s] %s\n", i+1, step.Tool, step.Purpose)
		for _, q := range step.Queries {
			fmt.Printf("     - %s\n", q)
		}
	}
	fmt.Printf("  목표: 출처 %d개↑ · 품질 %.1f↑", researchPlan.Goal.MinSources, researchPlan.Goal.MinScore)
	if researchPlan.Goal.NeedStats   { fmt.Print(" · 수치 필요") }
	if researchPlan.Goal.NeedPapers  { fmt.Print(" · 논문 필요") }
	if researchPlan.Goal.NeedRecent  { fmt.Print(" · 최신 자료 필요") }
	fmt.Println()
	
	// 2단계: 첫 번째 Tool 실행
	fmt.Println("\n[2단계] 초기 검색...")
	step2Start := time.Now()

	var allResults []tools.SearchResult
	var usedTools []string
	totalQueries := 0

	if len(researchPlan.Steps) > 0 {
		firstStep := researchPlan.Steps[0]
		firstAction := models.AgentAction{
			Action:  firstStep.Tool,
			Queries: firstStep.Queries,
			Reason:  firstStep.Purpose,
		}

		totalQueries += len(firstAction.Queries)
		results, err := a.execute(ctx, firstAction)
		if err != nil {
			fmt.Printf("  ⚠ 초기 검색 실패: %v\n", err)
		} else {
			allResults = append(allResults, results...)
			usedTools = append(usedTools, firstStep.Tool)
		}
	}
	step2Time := time.Since(step2Start)

	// 필터링 + 중복 제거
	filtered, blocked, lowScore := a.filterResults(allResults)
	filtered = deduplicateResults(filtered)
	fmt.Printf("  → 전체 %d개 중 유효 %d개 (차단: %d, 저품질: %d)\n",
		len(allResults), len(filtered), blocked, lowScore)

	// 3단계: Agent Loop
	fmt.Println("\n[3단계] Agent Loop...")
	fmt.Println(strings.Repeat("─", 50))
	loopStart := time.Now()

	for loop := 1; loop <= maxLoop; loop++ {
		// 현재 상태 평가
		eval := evaluate(filtered, researchPlan.Goal)

		fmt.Printf("\n  🔄 Loop %d/3\n", loop)
		fmt.Printf("     출처: %d개 | 품질: %.2f | 최신: %d개 | 수치: %v | 논문: %v\n",
			eval.SourceCount, eval.AvgScore,
			eval.RecentCount, eval.HasStats, eval.HasPapers)

		if len(eval.Missing) > 0 {
			fmt.Printf("     부족: %s\n", strings.Join(eval.Missing, ", "))
		}

		// 다음 행동 결정
		action, err := a.decide(ctx, topic, eval, researchPlan.Goal, usedTools, loop)
		if err != nil {
			fmt.Printf("  ⚠ 판단 실패: %v\n", err)
			break
		}

		fmt.Printf("     → [%s] %s\n", action.Action, action.Reason)

		if action.Action == "finish" {
			fmt.Println("     ✅ 완료")
			break
		}

		// Tool 실행
		totalQueries += len(action.Queries)
		newResults, err := a.execute(ctx, action)
		if err != nil {
			fmt.Printf("     ⚠ 실패: %v\n", err)
			continue
		}

		// 필터링 + 중복 제거 후 추가
		newFiltered, _, _ := a.filterResults(newResults)
		newFiltered = deduplicateResults(append(filtered, newFiltered...))
		added := len(newFiltered) - len(filtered)
		filtered = newFiltered
		usedTools = append(usedTools, action.Action)

		fmt.Printf("     → %d개 추가 (총 %d개)\n", added, len(filtered))
	}

	loopTime := time.Since(loopStart)
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("  완료 (%.1fs) | 최종 출처: %d개 | 사용 Tool: %s\n",
		loopTime.Seconds(), len(filtered), strings.Join(usedTools, " → "))

	// 품질 저장
	avgScore, maxScore, minScore := calcQualityStats(filtered)
	fmt.Printf("  품질: 평균 %.2f / 최고 %.2f / 최저 %.2f\n",
		avgScore, maxScore, minScore)
	a.saveQuality(topic, avgScore, maxScore, minScore)

	// 4단계: 리포트 작성
	fmt.Println("\n[4단계] 리포트 작성 중...")
	step3Start := time.Now()
	markdown, err := a.generateReport(ctx, topic, filtered)
	step3Time := time.Since(step3Start)
	if err != nil {
		return nil, fmt.Errorf("리포트 생성 실패: %w", err)
	}

	// 5단계: 카드뉴스 추출
	fmt.Println("\n[5단계] 카드뉴스 데이터 추출 중...")
	step4Start := time.Now()
	cards, err := a.extractCards(ctx, markdown)
	step4Time := time.Since(step4Start)
	if err != nil {
		fmt.Printf("  ⚠ 카드 추출 실패: %v\n", err)
	}

	// Breakdown
	a.printBreakdown(step1Time, step2Time, loopTime, step3Time, step4Time,
		time.Since(totalStart), avgScore, totalQueries, usedTools)

	return &models.Report{
		Topic:    topic,
		Markdown: markdown,
		Cards:    cards,
		Sources:  collectSources(filtered),
	}, nil
}

// ────────────────────────────────────────
// 내부 헬퍼 함수들
// ────────────────────────────────────────

// filterResults — 신뢰도 낮은 결과 제거
func (a *Agent) filterResults(results []tools.SearchResult) (filtered []tools.SearchResult, blockedCount, lowScoreCount int) {
	for _, r := range results {
		if a.isBlockedDomain(r.URL) {
			blockedCount++
			continue
		}
		if r.Score < a.cfg.MinScore {
			lowScoreCount++
			continue
		}
		filtered = append(filtered, r)
	}

	return filtered, blockedCount, lowScoreCount
}

// saveQuality — 검색 품질 저장
func (a *Agent) saveQuality(topic string, avgScore, maxScore, minScore float64) {
	if err := renderer.SaveQuality(models.QualityResult{
		Topic:    topic,
		Version:  a.cfg.PromptVersion,
		Note:     a.cfg.PromptNote,
		AvgScore: avgScore,
		MaxScore: maxScore,
		MinScore: minScore,
		RunAt:    time.Now().Format("2006-01-02 15:04:05"),
	}); err != nil {
		fmt.Printf("  ⚠ quality.md 저장 실패: %v\n", err)
	}
}

// printBreakdown — 소요 시간 출력
func (a *Agent) printBreakdown(s1, s2, s3, s4, s5, total time.Duration, avgScore float64, queryCount int, usedTools []string,) {
	fmt.Println()
    fmt.Println(strings.Repeat("─", 50))
    fmt.Println("⏱ 소요 시간 Breakdown")
    fmt.Println(strings.Repeat("─", 50))

    fmt.Printf("  [1/5] 계획 수립:   %5.1fs  (Gemini API)\n", s1.Seconds())
    fmt.Printf("  [2/5] 초기 검색:   %5.1fs\n", s2.Seconds())
    fmt.Printf("  [3/5] Agent Loop:  %5.1fs  (추가 검색)\n", s3.Seconds())
    fmt.Printf("  [4/5] 리포트 작성: %5.1fs  (Gemini API)\n", s4.Seconds())
    fmt.Printf("  [5/5] 카드 추출:   %5.1fs  (Gemini API)\n", s5.Seconds())

    fmt.Println(strings.Repeat("─", 50))
    fmt.Printf("  전체 검색 쿼리:   %d개\n", queryCount)
    fmt.Printf("  사용 Tool:        %s\n", strings.Join(usedTools, " → "))
    fmt.Printf("  총 소요 시간:     %5.1fs\n", total.Seconds())
    fmt.Printf("  검색 품질 점수:   %.2f / 1.0\n", avgScore)
    fmt.Println(strings.Repeat("─", 50))
}

// collectSources — 출처 URL 수집 (중복 제거)
func collectSources(results []tools.SearchResult) []string {
	var sources []string
	seen := map[string]bool{}
	for _, r := range results {
		if !seen[r.URL] {
			sources = append(sources, r.URL)
			seen[r.URL] = true
		}
	}
	return sources
}

// isBlockedDomain — config 기반 차단 도메인 확인
func (a *Agent) isBlockedDomain(url string) bool {
	for _, domain := range a.cfg.BlockedDomains {
		if strings.Contains(url, domain) {
			return true
		}
	}
	return false
}

// calcQualityStats — 검색 품질 통계 계산
func calcQualityStats(results []tools.SearchResult) (avg, max, min float64) {
	if len(results) == 0 {
		return 0, 0, 0
	}

	min = results[0].Score
	max = results[0].Score

	var total float64
	for _, r := range results {
		total += r.Score
		if r.Score > max {
			max = r.Score
		}
		if r.Score < min {
			min = r.Score
		}
	}

	avg = total / float64(len(results))
	return
}

// ────────────────────────────────────────
// 벤치마크
// ────────────────────────────────────────

// BenchmarkParallel — 병렬 vs 순차 실행 시간 비교
func (a *Agent) BenchmarkParallel(topic string) error {
	ctx := context.Background()

	fmt.Println("\n📊 병렬 vs 순차 벤치마크 시작")
	fmt.Println(strings.Repeat("─", 50))

	// 1. 동일한 쿼리 생성
	fmt.Println("\n쿼리 생성 중...")
	queries, err := a.generateQueries(ctx, topic)
	if err != nil {
		return fmt.Errorf("쿼리 생성 실패: %w", err)
	}

	fmt.Printf("쿼리 %d개 생성 완료\n", len(queries))
	for i, q := range queries {
		fmt.Printf("    %d. %s\n", i+1, q)
	}
	fmt.Println()

	// 2. 순차 실행
	fmt.Println("\n[1/2] 순차 실행 중...")
	seqStart := time.Now()
	_, err = a.orch.RunSequential(ctx, queries)
	seqTime := time.Since(seqStart)

	if err != nil {
		return fmt.Errorf("순차 실행 실패: %w", err)
	}
	fmt.Printf("완료: %dms\n", seqTime.Milliseconds())

	// 3. 병렬 실행
	fmt.Println("\n[2/2] 병렬 실행 중...")
	parStart := time.Now()
	_, err = a.orch.RunParallel(ctx, queries)
	parTime := time.Since(parStart)

	if err != nil {
		return fmt.Errorf("병렬 실행 실패: %w", err)
	}
	fmt.Printf("완료: %dms\n", parTime.Milliseconds())

	// ── 결과 출력 ──
	improvement := float64(seqTime) / float64(parTime)
	reduction := (1 - float64(parTime)/float64(seqTime)) * 100

	fmt.Println()
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println("📊 결과")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("  순차 실행: %dms\n", seqTime.Milliseconds())
	fmt.Printf("  병렬 실행: %dms\n", parTime.Milliseconds())
	fmt.Printf("  개선율:    %.1f배 빠름\n", improvement)
	fmt.Printf("  시간 단축: %.0f%%\n", reduction)
	fmt.Println(strings.Repeat("─", 50))

	// ── 파일 저장 ──
	if err := renderer.SaveBenchmark(models.BenchmarkResult{
		Topic:       topic,
		SeqTime:     seqTime.Milliseconds(),
		ParTime:     parTime.Milliseconds(),
		Workers:     a.cfg.MaxWorkers,
		Improvement: improvement,
		Reduction:   reduction,
		RunAt:       time.Now().Format("2006-01-02 15:04:05"),
	}); err != nil {
		return fmt.Errorf("벤치마크 저장 실패: %w", err)
	}

	fmt.Println("✅ benchmark.md 저장 완료!")
	fmt.Println("📄 저장 위치: output/benchmark.md")
	return nil
}

// BenchmarkWorkers — 워커 수별 성능 비교 (1, 2, 3, 4, 6)
func (a *Agent) BenchmarkWorkers(topic string) error {
	ctx := context.Background()

	fmt.Println("\n⚡ 워커 수별 벤치마크 시작")
	fmt.Println(strings.Repeat("─", 50))

	// 쿼리 생성
	fmt.Println("쿼리 생성 중...")
	queries, err := a.generateQueries(ctx, topic)
	if err != nil {
		return fmt.Errorf("쿼리 생성 실패: %w", err)
	}
	fmt.Printf("쿼리 %d개 생성 완료\n\n", len(queries))
	for i, q := range queries {
		fmt.Printf("  %d. %s\n", i+1, q)
	}
	fmt.Println()

	// 순차 실행 (기준값)
	fmt.Println("[기준] 순차 실행 중...")
	seqResult, err := a.runWithMetrics(ctx, 1, queries, nil)

	if err != nil {
		if saveErr := renderer.SaveTradeoffBenchmark(models.TradeoffBenchmarkResult{
			Topic:   topic,
			RunAt:   time.Now().Format("2006-01-02 15:04:05"),
			Results: []models.TradeoffResult{seqResult},
		}); saveErr != nil {
			fmt.Printf("  ⚠ 저장 실패: %v\n", saveErr)
		}
		return fmt.Errorf("순차 실행 실패: %w", err)
	}
	fmt.Printf("완료: %dms | 메모리: %.2fMB\n\n", seqResult.ElapsedMs, seqResult.MemoryMB)

	// 워커별 병렬 실행
	workerCounts := []int{2, 3, 4, 6}
	var tradeoffResults []models.TradeoffResult

	for _, w := range workerCounts {
		fmt.Printf("[워커 %d개] 병렬 실행 중...\n", w)
		tempOrch := orchestrator.New(tools.NewSearcher(a.cfg.TavilyAPIKey), w)
		result, err := a.runWithMetrics(ctx, w, queries, tempOrch)
		tradeoffResults = append(tradeoffResults, result)

		if err != nil {
			fmt.Printf("  실패: %v\n\n", err)
			continue
		}

		improvement := float64(seqResult.ElapsedMs) / float64(result.ElapsedMs)
		fmt.Printf("  완료: %dms | 메모리: %.2fMB | 순차 대비 %.1f배\n\n",
			result.ElapsedMs, result.MemoryMB, improvement)
	}

	// 결과 출력
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println("⚡ 워커 수별 결과")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("  %-12s %10dms  기준\n", "1 (순차)", seqResult.ElapsedMs)
	for _, r := range tradeoffResults {
		if r.Failed {
			fmt.Printf("  %-12s %10dms  실패\n",
				fmt.Sprintf("워커 %d개", r.Workers), r.ElapsedMs)
			continue
		}
		improvement := float64(seqResult.ElapsedMs) / float64(r.ElapsedMs)
		fmt.Printf("  %-12s %10dms  %.1f배\n",
			fmt.Sprintf("워커 %d개", r.Workers), r.ElapsedMs, improvement)
	}
	fmt.Println(strings.Repeat("─", 50))

	// 결과 저장
	allResults := append([]models.TradeoffResult{seqResult}, tradeoffResults...)
	if err := renderer.SaveTradeoffBenchmark(models.TradeoffBenchmarkResult{
		Topic:   topic,
		RunAt:   time.Now().Format("2006-01-02 15:04:05"),
		Results: allResults,
	}); err != nil {
		return fmt.Errorf("워커 벤치마크 저장 실패: %w", err)
	}

	fmt.Println("\n✅ benchmark.md 워커별 결과 저장 완료!")
	fmt.Println("📄 저장 위치: output/benchmark.md")
	return nil
}

// runWithMetrics — 실행 시간 + 메모리 측정 헬퍼
// orch가 nil이면 순차 실행
func (a *Agent) runWithMetrics(ctx context.Context, workers int, queries []string, orch *orchestrator.Orchestrator) (models.TradeoffResult, error) {
	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	start := time.Now()
	var err error

	if orch == nil {
		_, err = a.orch.RunSequential(ctx, queries)
	} else {
		_, err = orch.RunParallel(ctx, queries)
	}

	elapsed := time.Since(start)

	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)
	memMB := float64(memAfter.TotalAlloc-memBefore.TotalAlloc) / 1024 / 1024

	return models.TradeoffResult{
		Workers:   workers,
		ElapsedMs: elapsed.Milliseconds(),
		MemoryMB:  memMB,
		Failed:    err != nil,
	}, err
}

// ────────────────────────────────────────
// LLM 호출
// ────────────────────────────────────────

// generateQueries — Gemini로 검색 쿼리 생성
func (a *Agent) generateQueries(ctx context.Context, topic string) ([]string, error) {
	prompt := fmt.Sprintf(`다음 주제를 다양한 각도로 검색할 쿼리를 %d개 생성해주세요.
주제: %s

규칙:
- 한 줄에 쿼리 하나씩
- 번호나 기호 없이 쿼리 텍스트만
- 각 쿼리는 서로 다른 검색 의도를 가져야 함
- 동일하거나 의미가 유사한 쿼리는 생성하지 말 것
- 영어/한국어 혼용 가능
- 최신 정보를 찾을 수 있게 구체적으로`, a.cfg.MaxSearches, topic)

	resp, err := a.callGemini(ctx, prompt)
	if err != nil {
		return nil, err
	}

	var queries []string
	for _, line := range strings.Split(resp, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			queries = append(queries, line)
		}
	}

	if len(queries) > a.cfg.MaxSearches {
		queries = queries[:a.cfg.MaxSearches]
	}

	return queries, nil
}

// generateReport — 검색 결과로 마크다운 리포트 작성
func (a *Agent) generateReport(ctx context.Context, topic string, results []tools.SearchResult) (string, error) {
	var sb strings.Builder
	for i, r := range results {
		if i >= 15 {
			break
		}
		sb.WriteString(fmt.Sprintf(
			"### 출처 %d: %s\nURL: %s\n%s\n\n",
			i+1, r.Title, r.URL, r.Content,
		))
	}

	prompt := fmt.Sprintf(`다음 검색 결과를 바탕으로 "%s" 주제의 리포트를 작성해주세요.

=== 검색 결과 ===
%s

=== 작성 규칙 ===
- 마크다운 형식
- 구성: 제목 → 요약 → 주요내용(3~5개 섹션) → 결론 → 출처
- 각 섹션마다 소제목과 2~3 문단
- 출처 URL 포함
- 한국어로 작성
- 객관적이고 사실 중심으로 간결하고 핵심적으로`, topic, sb.String())

	return a.callGemini(ctx, prompt)
}

// extractCards — 리포트에서 카드뉴스 데이터 추출
func (a *Agent) extractCards(ctx context.Context, markdown string) ([]models.CardContent, error) {
	prompt := fmt.Sprintf(`다음 리포트에서 카드뉴스 6장을 만들 데이터를 추출해주세요.

리포트:
%s

아래 JSON 배열 형식으로만 응답하세요. 다른 텍스트는 절대 포함하지 마세요.
[
  {
    "card_number": 1,
    "headline": "15자 이내 핵심 제목",
    "body": "2~3문장 핵심 내용. 구체적 수치 포함.",
    "emoji": "이모지 1개",
    "tag": "트렌드 또는 핵심 또는 데이터 또는 전망 또는 사례 또는 인사이트"
  }
]

카드 구성:
- 카드 1: 전체 요약 (훅)
- 카드 2~5: 핵심 인사이트
- 카드 6: 결론/전망`, markdown)

	resp, err := a.callGemini(ctx, prompt)
	if err != nil {
		return nil, err
	}

	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var raw []struct {
		CardNumber int    `json:"card_number"`
		Headline   string `json:"headline"`
		Body       string `json:"body"`
		Emoji      string `json:"emoji"`
		Tag        string `json:"tag"`
	}
	if err := json.Unmarshal([]byte(resp), &raw); err != nil {
		return nil, fmt.Errorf("카드 JSON 파싱 실패: %w", err)
	}

	var cards []models.CardContent
	for _, r := range raw {
		cards = append(cards, models.CardContent{
			CardNumber: r.CardNumber,
			Headline:   r.Headline,
			Body:       r.Body,
			Emoji:      r.Emoji,
			Tag:        r.Tag,
		})
	}

	return cards, nil
}

// callGemini — Gemini API 실제 호출
func (a *Agent) callGemini(ctx context.Context, prompt string) (string, error) {
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-3.5-flash-lite:generateContent?key=%s",
		a.cfg.GeminiAPIKey,
	)

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: prompt}}},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("요청 JSON 생성 실패: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Gemini API 호출 실패: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Gemini API 오류 (status=%d)", resp.StatusCode)
	}

	var geminiResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
    	return "", fmt.Errorf("Gemini 응답 파싱 실패: %w", err)
	}
	
	if len(geminiResp.Candidates) == 0 ||
		len(geminiResp.Candidates[0].Content.Parts) == 0 {

		rawBody, _ := json.Marshal(geminiResp)
    	return "", fmt.Errorf("Gemini 응답이 비어있음. 실제 응답: %s", string(rawBody))
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}