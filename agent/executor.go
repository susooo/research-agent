package agent

import (
	"context"
	"fmt"

	"research-agent/models"
	"research-agent/orchestrator"
	"research-agent/tools"
)

// execute — Tool 실행 (병렬)
func (a *Agent) execute(ctx context.Context, action models.AgentAction) ([]tools.SearchResult, error) {
	switch action.Action {

	case "tavily":
		fmt.Printf("     🔍 Tavily 검색 (%d개 쿼리, 병렬)...\n", len(action.Queries))
		searcher := tools.NewSearcher(a.cfg.TavilyAPIKey)
		orch := orchestrator.New(searcher, a.cfg.MaxWorkers)
		return orch.RunParallel(ctx, action.Queries)

	case "exa":
		fmt.Printf("     🔬 Exa 검색 (%d개 쿼리, 병렬)...\n", len(action.Queries))
		return a.runExaParallel(ctx, action.Queries)

	case "scholar":
		fmt.Printf("     📚 Scholar 검색 (%d개 쿼리, 병렬)...\n", len(action.Queries))
		return a.runScholarParallel(ctx, action.Queries)
	}

	return nil, fmt.Errorf("알 수 없는 Tool: %s", action.Action)
}

// runExaParallel — Exa 병렬 실행
func (a *Agent) runExaParallel(ctx context.Context, queries []string) ([]tools.SearchResult, error) {
	type result struct {
		results []tools.SearchResult
		err     error
	}

	ch := make(chan result, len(queries))

	for _, q := range queries {
		q := q
		go func() {
			res, err := a.exa.Search(q)
			ch <- result{res, err}
		}()
	}

	var allResults []tools.SearchResult
	for range queries {
		r := <-ch
		if r.err != nil {
			fmt.Printf("     ⚠ Exa 검색 실패: %v\n", r.err)
			continue
		}
		allResults = append(allResults, r.results...)
	}

	return allResults, nil
}

// runScholarParallel — Semantic Scholar 병렬 실행
func (a *Agent) runScholarParallel(ctx context.Context, queries []string) ([]tools.SearchResult, error) {
	type result struct {
		results []tools.SearchResult
		err     error
	}

	ch := make(chan result, len(queries))
	scholar := tools.NewSemanticScholarSearcher()

	for _, q := range queries {
		q := q
		go func() {
			res, err := scholar.Search(q)
			ch <- result{res, err}
		}()
	}

	var allResults []tools.SearchResult
	for range queries {
		r := <-ch
		if r.err != nil {
			fmt.Printf("     ⚠ Scholar 검색 실패: %v\n", r.err)
			continue
		}
		allResults = append(allResults, r.results...)
	}

	return allResults, nil
}