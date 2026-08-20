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

	"research-agent/config"
	"research-agent/orchestrator"
	"research-agent/tools"
)

// Report — 최종 결과물
type Report struct {
	Topic    string
	Markdown string
	Cards    []CardContent
	Sources  []string
}

// CardContent — 카드뉴스 한장
type CardContent struct {
	CardNumber int
	Headline   string
	Body       string
	Emoji      string
	Tag        string
}

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
func (a *Agent) Research(topic string) (*Report, error) {
	ctx := context.Background()

	// 1단계: 검색 쿼리 생성
	fmt.Println("\n[1/4] 검색 쿼리 생성 중...")
	queries, err := a.generateQueries(ctx, topic)
	if err != nil {
		return nil, fmt.Errorf("쿼리 생성 실패: %w", err)
	}

	// 2단계: 병렬 웹 검색
	fmt.Printf("\n[2/4] 병렬 웹 검색 중 (%d개 Goroutine)...\n", a.cfg.MaxWorkers)
	start := time.Now()
	results, err := a.orch.RunParallel(ctx, queries)
	if err != nil {
		return nil, fmt.Errorf("웹 검색 실패: %w", err)
	}
	fmt.Printf("  → 총 %d개 결과 수집 (%.1fs)\n",
		len(results), time.Since(start).Seconds())

	// 3단계: 리포트 작성
	fmt.Println("\n[3/4] 리포트 작성 중...")
	markdown, err := a.generateReport(ctx, topic, results)
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
	for _, r := range results {
		if !seen[r.URL] {
			sources = append(sources, r.URL)
			seen[r.URL] = true
		}
	}

	return &Report{
		Topic:    topic,
		Markdown: markdown,
		Cards:    cards,
		Sources:  sources,
	}, nil
}

// generateQueries — Gemini로 검색 쿼리 생성
func (a *Agent) generateQueries(ctx context.Context, topic string) ([]string, error) {
	prompt := fmt.Sprintf(`다음 주제를 다양한 각도로 검색할 쿼리를 %d개 생성해주세요.
주제: %s

규칙:
- 한 줄에 쿼리 하나씩
- 번호나 기호 없이 쿼리 텍스트만
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
func (a *Agent) extractCards(ctx context.Context, markdown string) ([]CardContent, error) {
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

	var cards []CardContent
	for _, r := range raw {
		cards = append(cards, CardContent{
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