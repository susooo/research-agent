package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"research-agent/tools"
)

const (
	colorReset = "\033[0m"
	colorGreen = "\033[32m"
	colorRed   = "\033[31m"
)

type Orchestrator struct {
	searcher   *tools.TavilySearcher
	maxWorkers int
}

type job struct {
	index int
	query string
}

type result struct {
	index   int
	results []tools.SearchResult
	err     error
}

func New(searcher *tools.TavilySearcher, maxWorkers int) *Orchestrator {
	return &Orchestrator{
		searcher:   searcher,
		maxWorkers: maxWorkers,
	}
}

// queries 슬라이스를 받아서 Goroutine Pool로 동시에 검색
func (o *Orchestrator) RunParallel(ctx context.Context, queries []string) ([]tools.SearchResult, error) {
	//채널 만들기
	jobs := make(chan job, len(queries))
	results := make(chan result, len(queries))

	// 작업 채널에 쿼리 넣기
	for i, q := range queries {
		jobs <- job{index: i, query: q}
	}
	close(jobs)

	//Goroutine Pool 실행
	//go 키워드 하나로 동시 실행 — Python async 없이
	workerCount := o.maxWorkers
	if workerCount > len(queries) {
		workerCount = len(queries)
	}

	//모든 Goroutine이 끝날 때까지 기다리는 카운터
	var wg sync.WaitGroup
	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				select {
				case <-ctx.Done():
					results <- result{index: j.index, err: ctx.Err()}
					return
				default:
				}
				
				start := time.Now()
				searchResults, err := o.searcher.Search(j.query)
				elapsed := time.Since(start)

				if err != nil {
					fmt.Printf(colorRed+"  ✗ [%dms] %s → 실패"+colorReset+"\n",
						elapsed.Milliseconds(), j.query)
				} else {
					fmt.Printf(colorGreen+"  ✓ [%dms] %s → %d개 결과"+colorReset+"\n",
						elapsed.Milliseconds(), j.query, len(searchResults))
				}

				results <- result{
					index:   j.index,
					results: searchResults,
					err:     err,
				}
			}
		}()
	}

	//모든 워커가 끝나면 results 채널 닫기
	go func() {
		wg.Wait()
		close(results)
	}()

	//결과 수집
	var allResults []tools.SearchResult
	var errCount int

	for r := range results {
		if r.err != nil {
			errCount++
			continue
		}
		allResults = append(allResults, r.results...)
	}

	//전부 실패한 경우만 에러 반환 (일부 실패는 허용)
	if errCount == len(queries) {
		return nil, fmt.Errorf("모든 검색이 실패했습니다")
	}

	return allResults, nil
}

// RunSequential — 순차 실행 (벤치마크 비교용)
func (o *Orchestrator) RunSequential(ctx context.Context, queries []string) ([]tools.SearchResult, error) {
	var allResults []tools.SearchResult

	for _, query := range queries {
		start := time.Now()
		results, err := o.searcher.Search(query)
		elapsed := time.Since(start)

		if err != nil {
			fmt.Printf(colorRed+"  ✗ [%dms] %s → 실패"+colorReset+"\n",
				elapsed.Milliseconds(), query)
			continue
		}
		fmt.Printf(colorGreen+"  ✓ [%dms] %s → %d개 결과"+colorReset+"\n",
			elapsed.Milliseconds(), query, len(results))

		allResults = append(allResults, results...)
	}

	return allResults, nil
}