package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SemanticScholarSearcher struct {
	client *http.Client
}

type semanticScholarResponse struct {
	Data []struct {
		PaperID  string `json:"paperId"`
		Title    string `json:"title"`
		Abstract string `json:"abstract"`
		Year     int    `json:"year"`
		URL      string `json:"url"`
		Authors  []struct {
			Name string `json:"name"`
		} `json:"authors"`
		CitationCount int `json:"citationCount"`
	} `json:"data"`
}

func NewSemanticScholarSearcher() *SemanticScholarSearcher {
	return &SemanticScholarSearcher{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *SemanticScholarSearcher) Search(query string) ([]SearchResult, error) {
	reqURL := fmt.Sprintf(
		"https://api.semanticscholar.org/graph/v1/paper/search?query=%s&fields=title,abstract,year,url,authors,citationCount&limit=5",
		url.QueryEscape(query),
	)

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "research-agent/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Scholar 요청 실패: %w", err)
	}
	defer resp.Body.Close()

	var ssResp semanticScholarResponse
	if err := json.NewDecoder(resp.Body).Decode(&ssResp); err != nil {
		return nil, fmt.Errorf("Scholar 응답 파싱 실패: %w", err)
	}

	var results []SearchResult
	for _, p := range ssResp.Data {
		var authors []string
		for _, a := range p.Authors {
			authors = append(authors, a.Name)
		}

		paperURL := p.URL
		if paperURL == "" {
			paperURL = "https://www.semanticscholar.org/paper/" + p.PaperID
		}

		content := fmt.Sprintf(
			"[%d년] 인용 %d회 | 저자: %s\n%s",
			p.Year,
			p.CitationCount,
			strings.Join(authors, ", "),
			p.Abstract,
		)

		results = append(results, SearchResult{
			Query:   query,
			Title:   p.Title,
			URL:     paperURL,
			Content: content,
			Score:   calcPaperScore(p.CitationCount),
		})
	}

	return results, nil
}

func calcPaperScore(citations int) float64 {
	switch {
	case citations >= 1000:
		return 0.99
	case citations >= 100:
		return 0.95
	case citations >= 10:
		return 0.85
	case citations >= 1:
		return 0.75
	default:
		return 0.65
	}
}