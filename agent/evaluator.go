package agent

import (
	"strings"

	"research-agent/models"
	"research-agent/tools"
)

// evaluate — 검색 결과 평가
func evaluate(results []tools.SearchResult, goal models.PlanGoal) models.EvalResult {
	eval := models.EvalResult{}

	if len(results) == 0 {
		eval.Missing = []string{"출처 없음"}
		return eval
	}

	eval.SourceCount = len(results)

	// 평균 Score
	var total float64
	for _, r := range results {
		total += r.Score

		// 최신 자료
		if strings.Contains(r.Content, "2025") ||
			strings.Contains(r.Content, "2026") ||
			strings.Contains(r.Title, "2025") ||
			strings.Contains(r.Title, "2026") {
			eval.RecentCount++
		}

		// 수치/통계
		if !eval.HasStats {
			for _, kw := range []string{"%", "억", "조", "million", "billion", "$"} {
				if strings.Contains(r.Content, kw) {
					eval.HasStats = true
					break
				}
			}
		}

		// 논문 포함
		if !eval.HasPapers {
			for _, domain := range []string{
				"arxiv.org", "semanticscholar.org",
				"scholar.google", "ieee.org", "acm.org",
			} {
				if strings.Contains(r.URL, domain) {
					eval.HasPapers = true
					break
				}
			}
		}
	}
	eval.AvgScore = total / float64(len(results))

	// 부족한 것 파악
	if eval.SourceCount < goal.MinSources {
		eval.Missing = append(eval.Missing, "출처 부족")
	}
	if eval.AvgScore < goal.MinScore {
		eval.Missing = append(eval.Missing, "품질 미달")
	}
	if goal.NeedStats && !eval.HasStats {
		eval.Missing = append(eval.Missing, "수치/통계 없음")
	}
	if goal.NeedPapers && !eval.HasPapers {
		eval.Missing = append(eval.Missing, "논문 없음")
	}
	if goal.NeedRecent && eval.RecentCount < 2 {
		eval.Missing = append(eval.Missing, "최신 자료 부족")
	}

	eval.GoalMet = len(eval.Missing) == 0
	return eval
}

// deduplicateResults — 중복 결과 제거
func deduplicateResults(results []tools.SearchResult) []tools.SearchResult {
	seen := map[string]bool{}
	var deduped []tools.SearchResult

	for _, r := range results {
		// URL 중복 제거
		if seen[r.URL] {
			continue
		}
		seen[r.URL] = true
		deduped = append(deduped, r)
	}

	return deduped
}