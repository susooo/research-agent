package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	// "io"
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
	}
}

// Research — 전체 리서치 흐름
func (a *Agent) Research(topic string) (*models.Report, error) {
	ctx := context.Background()

	// 1단계: 검색 쿼리 생성
	fmt.Println("\n[1/4] 검색 쿼리 생성 중...")
	queries, err := a.generateQueries(ctx, topic)
	if err != nil {
		return nil, fmt.Errorf("쿼리 생성 실패: %w", err)
	}

	// 2단계: 병렬 웹 검색
	fmt.Printf("\n[2/4] 병렬 웹 검색 중 (%d개 Goroutine)...\n", a.cfg.MaxWorkers)
	//start := time.Now()
	results, err := a.orch.RunParallel(ctx, queries)
	if err != nil {
		return nil, fmt.Errorf("웹 검색 실패: %w", err)
	}

	// 2-1단계: 신뢰도 낮은 출처 차단
	var filteredResults []tools.SearchResult
	var blockedCount, lowScoreCount int

	for _, r := range results {
		// 차단 도메인 확인
		if a.isBlockedDomain(r.URL) {
			blockedCount++
			continue
		}
		// Score 임계값 확인
		if r.Score < a.cfg.MinScore {
			lowScoreCount++
			continue
		}
		filteredResults = append(filteredResults, r)
	}

	fmt.Printf("  → 전체 %d개 중 신뢰 결과 %d개 사용\n",
		len(results), len(filteredResults))
	fmt.Printf("    차단 도메인: %d개 / Score 미달(%.1f 미만): %d개\n",
		blockedCount, a.cfg.MinScore, lowScoreCount)

	// 결과 너무 적으면 Score 기준 완화 (차단 도메인은 유지)
	if len(filteredResults) < 5 {
		fmt.Println("  ⚠ 결과 부족 → Score 기준 0.5로 완화")
		filteredResults = nil
		for _, r := range results {
			if a.isBlockedDomain(r.URL) {
				continue
			}
			if r.Score >= 0.5 {
				filteredResults = append(filteredResults, r)
			}
		}
		fmt.Printf("  → 완화 후 %d개 사용\n", len(filteredResults))
	}

	// 2-2단계: 검색 품질 평가
	avgScore, maxScore, minScore := calcQualityStats(filteredResults)
	fmt.Printf("  → 검색 품질 점수: 평균 %.2f / 최고 %.2f / 최저 %.2f\n", avgScore, maxScore, minScore)

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

	// 3단계: 리포트 작성
	fmt.Println("\n[3/4] 리포트 작성 중...")
	markdown, err := a.generateReport(ctx, topic, filteredResults)
	if err != nil {
		return nil, fmt.Errorf("리포트 생성 실패: %w", err)
	}

	// 4단계: 카드뉴스 데이터 추출
	fmt.Println("\n[4/4] 카드뉴스 데이터 추출 중...")
	cards, err := a.extractCards(ctx, markdown)
	if err != nil {
		// 카드 실패는 치명적이지 않음 — 리포트는 살림
		fmt.Printf("  ⚠ 카드 추출 실패 (리포트는 정상): %v\n", err)
	}

	// 출처 URL 수집 (중복 제거)
	var sources []string
	seen := map[string]bool{}
	for _, r := range filteredResults {
		if !seen[r.URL] {
			sources = append(sources, r.URL)
			seen[r.URL] = true
		}
	}

	return &models.Report{
		Topic:    topic,
		Markdown: markdown,
		Cards:    cards,
		Sources:  sources,
	}, nil
}

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
	fmt.Println()

	err = renderer.SaveBenchmark(models.BenchmarkResult{
		Topic:       topic,
		SeqTime:     seqTime.Milliseconds(),
		ParTime:     parTime.Milliseconds(),
		Workers:     a.cfg.MaxWorkers,
		Improvement: improvement,
		Reduction:   reduction,
		RunAt:       time.Now().Format("2006-01-02 15:04:05"),
	})
	if err != nil {
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

	// ==============================
	// 1. 순차 실행 (기준값)
	// ==============================
	fmt.Println("[기준] 순차 실행 중...")

	runtime.GC()
	var seqMemBefore runtime.MemStats
	runtime.ReadMemStats(&seqMemBefore)

	seqStart := time.Now()
	_, seqErr := a.orch.RunSequential(ctx, queries)
	seqTime := time.Since(seqStart)
	
	var seqMemAfter runtime.MemStats
	runtime.ReadMemStats(&seqMemAfter)
	seqMemMB := float64(seqMemAfter.TotalAlloc-seqMemBefore.TotalAlloc) / 1024 / 1024 //Byte를 MB로 변환

	seqResult := models.TradeoffResult{
		Workers:   1,
		ElapsedMs: seqTime.Milliseconds(),
		MemoryMB:  seqMemMB,
		Failed:    seqErr != nil,
	}

	if seqErr != nil {
		fmt.Printf("실패: %v\n", seqErr)

		// 순차 실행 실패 결과 저장
		err := renderer.SaveTradeoffBenchmark(
			models.TradeoffBenchmarkResult{
				Topic:   topic,
				RunAt:   time.Now().Format("2006-01-02 15:04:05"),
				Results: []models.TradeoffResult{
					seqResult,
				},
			},
		)

		if err != nil {
			return fmt.Errorf("벤치마크 저장 실패: %w", err)
		}

		return fmt.Errorf("순차 실행 실패: %w", seqErr)
	}

	// ==============================
	// 2. Worker 수별 병렬 실행
	// ==============================
	workerCounts := []int{2, 3, 4, 6}
	var tradeoffResults []models.TradeoffResult

	for _, w := range workerCounts {
		fmt.Printf("[워커 %d개] 병렬 실행 중...\n", w)

		tempOrch := orchestrator.New(
			tools.NewSearcher(a.cfg.TavilyAPIKey),
			w,
		)

		runtime.GC()
		var memBefore runtime.MemStats
		runtime.ReadMemStats(&memBefore)

		start := time.Now()
		_, workerErr := tempOrch.RunParallel(ctx, queries)
		elapsed := time.Since(start)
		
		var memAfter runtime.MemStats
		runtime.ReadMemStats(&memAfter)
		memMB := float64(memAfter.TotalAlloc-memBefore.TotalAlloc) / 1024 / 1024

		result := models.TradeoffResult{
			Workers:   w,
			ElapsedMs: elapsed.Milliseconds(),
			MemoryMB:  memMB,
			Failed:    workerErr != nil,
		}

		tradeoffResults = append(
			tradeoffResults,
			result,
		)

		// 실패한 Worker는 기록만 하고 다음 Worker 진행
		if workerErr != nil {
			fmt.Printf("실패: %v\n\n", workerErr)
			continue
		}

		improvement := float64(seqTime) / float64(elapsed)

		fmt.Printf(
			"완료: %dms (순차 대비 %.1f배)\n\n",
			elapsed.Milliseconds(),
			improvement,
		)
	}

	// ==============================
	// 3. 간단한 결과 출력
	// ==============================
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println("⚡ 워커 수별 결과")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("  %-12s %10dms  기준\n", "1 (순차)", seqResult.ElapsedMs)
	for _, r := range tradeoffResults {
		if r.Failed {
			fmt.Printf(
				"  %-12s %10dms  실패\n",
				fmt.Sprintf("워커 %d개", r.Workers),
				r.ElapsedMs,
			)
			continue
		}

		improvement := float64(seqResult.ElapsedMs) / float64(r.ElapsedMs)

		fmt.Printf(
			"  %-12s %10dms  %.1f배\n",
			fmt.Sprintf("워커 %d개", r.Workers),
			r.ElapsedMs,
			improvement,
		)
	}
	fmt.Println(strings.Repeat("─", 50))

	// ==============================
	// 4. 전체 결과 저장
	// ==============================
	allResults := []models.TradeoffResult{
		seqResult,
	}

	allResults = append(allResults, tradeoffResults...)

	err = renderer.SaveTradeoffBenchmark(
		models.TradeoffBenchmarkResult{
			Topic:   topic,
			RunAt:   time.Now().Format("2006-01-02 15:04:05"),
			Results: allResults,
		},
	)

	if err != nil {
		return fmt.Errorf("워커 벤치마크 저장 실패: %w", err)
	}

	fmt.Println("\n✅ benchmark.md 워커별 결과 저장 완료!")
	fmt.Println("📄 저장 위치: output/benchmark.md")

	return nil
}

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

	// 응답을 줄 단위로 파싱
	var queries []string
	for _, line := range strings.Split(resp, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			queries = append(queries, line)
		}
	}

	// MaxSearches 초과하면 자르기
	if len(queries) > a.cfg.MaxSearches {
		queries = queries[:a.cfg.MaxSearches]
	}

	return queries, nil
}

// generateReport — 검색 결과로 마크다운 리포트 작성
func (a *Agent) generateReport(ctx context.Context, topic string, results []tools.SearchResult) (string, error) {
	// 검색 결과를 프롬프트에 담기 (최대 15개)
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

	// JSON 펜스 제거 (```json ... ``` 형태로 올 수 있음)
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	// JSON 파싱
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

	/*
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Gemini 응답 읽기 실패: %w", err)
	}

	fmt.Printf("Gemini 원본 응답: %s\n", string(bodyBytes))
	*/

	// HTTP 에러 먼저 확인
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf(
			"Gemini API 오류 (status=%d)",
			resp.StatusCode,
		)
	}

	var geminiResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
    	return "", fmt.Errorf("Gemini 응답 파싱 실패: %w", err)
	}
	/*
	if err := json.Unmarshal(bodyBytes, &geminiResp); err != nil {
    	return "", fmt.Errorf("Gemini 응답 파싱 실패: %w", err)
	}
	*/
	
	if len(geminiResp.Candidates) == 0 ||
		len(geminiResp.Candidates[0].Content.Parts) == 0 {

		rawBody, _ := json.Marshal(geminiResp)
    	return "", fmt.Errorf("Gemini 응답이 비어있음. 실제 응답: %s", string(rawBody))
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
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

// isBlockedDomain — config 기반 차단 도메인 확인
func (a *Agent) isBlockedDomain(url string) bool {
    for _, domain := range a.cfg.BlockedDomains {
        if strings.Contains(url, domain) {
            return true
        }
    }
    return false
}