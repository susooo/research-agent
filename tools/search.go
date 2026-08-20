package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type SearchResult struct {
	Query   string
	Title   string
	URL     string
	Content string
}

type TavilySearcher struct {
	APIKey string
	client *http.Client
}

type tavilyRequest struct {
	APIKey      string `json:"api_key"`
	Query       string `json:"query"`
	SearchDepth string `json:"search_depth"`
	MaxResults  int    `json:"max_results"`
}

type tavilyResponse struct {
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
	} `json:"results"`
}

func NewSearcher(apiKey string) *TavilySearcher {
	return &TavilySearcher{
		APIKey: apiKey,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// 단일 검색
// Goroutine에서 병렬로 호출
func (s *TavilySearcher) Search(query string) ([]SearchResult, error) {
	reqBody := tavilyRequest{
		APIKey:      s.APIKey,
		Query:       query,
		SearchDepth: "advanced",
		MaxResults:  3,
	}

	//구조체를 json으로 변환
	body, _ := json.Marshal(reqBody)
	resp, err := s.client.Post(
		"https://api.tavily.com/search",
		"application/json",
		bytes.NewBuffer(body), //HTTP가 이해하는 바이트 스트림 형태로 포장
	)
	if err != nil {
		return nil, fmt.Errorf("검색 요청 실패 [%s]: %w", query, err) //인터넷 끊김, Tavily 서버 다운 등
	}
	defer resp.Body.Close()

	var tavilyResp tavilyResponse
	if err := json.NewDecoder(resp.Body).Decode(&tavilyResp); err != nil {
		return nil, fmt.Errorf("응답 파싱 실패: %w", err)
	}

	var results []SearchResult
	for _, r := range tavilyResp.Results {
		results = append(results, SearchResult{
			Query:   query,
			Title:   r.Title,
			URL:     r.URL,
			Content: r.Content,
		})
	}

	return results, nil
}