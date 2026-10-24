package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type exaContents struct {
	Text exaText `json:"text"`
}

type exaText struct {
	MaxCharacters int `json:"maxCharacters"`
}

type ExaSearcher struct {
	APIKey string
	client *http.Client
}

type exaRequest struct {
	Query             string   `json:"query"`
	NumResults        int      `json:"numResults"`
	Type              string   `json:"type"`
	UseAutoprompt     bool     `json:"useAutoprompt"`
	Contents          exaContents `json:"contents"`
}

type exaResponse struct {
	Results []struct {
		Title   string  `json:"title"`
		URL     string  `json:"url"`
		Content string  `json:"content"`
		Score   float64 `json:"score"`
		Author  string  `json:"author"`
		PublishedDate string `json:"publishedDate"`
	} `json:"results"`
}

func NewExaSearcher(apiKey string) *ExaSearcher {
	return &ExaSearcher{
		APIKey: apiKey,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// Search — 시맨틱 검색 (학술/기술 콘텐츠 특화)
func (s *ExaSearcher) Search(query string) ([]SearchResult, error) {
	reqBody := exaRequest{
		Query:         query,
		NumResults:    5,
		Type:          "neural",   // 시맨틱 검색
		UseAutoprompt: true,       // 쿼리 자동 최적화
		Contents: exaContents{
			Text: exaText{
				MaxCharacters: 1000,
			},
		},
	}

	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST",
		"https://api.exa.ai/search",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", s.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Exa 요청 실패 [%s]: %w", query, err)
	}
	defer resp.Body.Close()

	var exaResp exaResponse
	if err := json.NewDecoder(resp.Body).Decode(&exaResp); err != nil {
		return nil, fmt.Errorf("Exa 응답 파싱 실패: %w", err)
	}

	var results []SearchResult
	for _, r := range exaResp.Results {
		// 날짜 + 저자 메타 정보 추가
		meta := ""
		if r.PublishedDate != "" {
			meta += "[" + r.PublishedDate[:10] + "] "
		}
		if r.Author != "" {
			meta += "저자: " + r.Author + " | "
		}

		content := strings.TrimSpace(meta + r.Content)

		results = append(results, SearchResult{
			Query:   query,
			Title:   r.Title,
			URL:     r.URL,
			Content: content,
			Score:   r.Score,
		})
	}

	return results, nil
}