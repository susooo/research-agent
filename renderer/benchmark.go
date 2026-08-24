package renderer

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"research-agent/models"
)

type BenchmarkResult struct {
	Topic       string
	SeqTime     int64
	ParTime     int64
	Workers     int
	Improvement float64
	Reduction   float64
	RunAt       string
}

// SaveBenchmark — 벤치마크 결과 누적 저장
func SaveBenchmark(result models.BenchmarkResult) error {
	path := "output/benchmark.md"

	existing := ""
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	}

	if existing == "" {
		existing = buildEmptyBenchmark()
	}

	// 새 행 추가
	newRow := fmt.Sprintf(
		"| %s | %s | %dms | %dms | %.1f배 | %.0f%% |\n",
		result.RunAt[:10], // 날짜만 (2026-08-20)
		result.Topic,
		result.SeqTime,
		result.ParTime,
		result.Improvement,
		result.Reduction,
	)

	// 표 마지막 행 뒤에 추가
	updated := strings.Replace(
		existing,
		"<!-- new row -->",
		newRow+"<!-- new row -->",
		1,
	)

	// 평균 재계산
	updated = recalculateAverage(updated)

	return os.WriteFile(path, []byte(updated), 0644)
}

// buildEmptyBenchmark — 빈 파일 초기 구조
func buildEmptyBenchmark() string {
	return `# 📈 벤치마크 기록

> Research Agent 병렬 vs 순차 실행 성능 측정 누적 기록


## 측정 결과

| 날짜 | 주제 | 순차 실행 | 병렬 실행 | 개선율 | 단축률 |
| --- | --- | --- | --- | --- | --- |
<!-- new row -->


## 📊 전체 평균

| 평균 순차 실행 | 평균 병렬 실행 | 평균 개선율 | 평균 단축률 | 총 측정 횟수 |
| --- | --- | --- | --- | --- |
| - | - | - | - | 0회 |

`
}

// recalculateAverage — 전체 행 읽어서 평균 재계산
func recalculateAverage(content string) string {
	lines := strings.Split(content, "\n")

	var seqTimes, parTimes, improvements, reductions []float64

	for _, line := range lines {
		if !isDataRow(line) {
			continue
		}

		cols := strings.Split(line, "|")
		if len(cols) < 7 {
			continue
		}

		seqStr := strings.TrimSuffix(strings.TrimSpace(cols[3]), "ms")
		parStr := strings.TrimSuffix(strings.TrimSpace(cols[4]), "ms")
		impStr := strings.TrimSuffix(strings.TrimSpace(cols[5]), "배")
		redStr := strings.TrimSuffix(strings.TrimSpace(cols[6]), "%")

		seq, err1 := strconv.ParseFloat(seqStr, 64)
		par, err2 := strconv.ParseFloat(parStr, 64)
		imp, err3 := strconv.ParseFloat(impStr, 64)
		red, err4 := strconv.ParseFloat(redStr, 64)

		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			continue
		}

		seqTimes = append(seqTimes, seq)
		parTimes = append(parTimes, par)
		improvements = append(improvements, imp)
		reductions = append(reductions, red)
	}

	count := len(seqTimes)
	if count == 0 {
		return content
	}

	// 평균 계산
	avgSeq := average(seqTimes)
	avgPar := average(parTimes)
	avgImp := average(improvements)
	avgRed := average(reductions)

	// 평균 행 교체
	newAvgRow := fmt.Sprintf(
		"| %.0fms | %.0fms | %.1f배 | %.0f%% | %d회 |",
		avgSeq, avgPar, avgImp, avgRed, count,
	)

	// 평균 행 찾기
	lines2 := strings.Split(content, "\n")
	inAverageSection := false
	
	for i, line := range lines2 {
		if strings.Contains(line, "## 📊 전체 평균") {
			inAverageSection = true
			continue
		}

		if !inAverageSection {
			continue
		}

		if strings.HasPrefix(line, "| ") &&
			!strings.Contains(line, "---") &&
			!strings.Contains(line, "평균 순차 실행") {

			lines2[i] = newAvgRow
			break
		}
	}

	return strings.Join(lines2, "\n")
}

func average(nums []float64) float64 {
	if len(nums) == 0 {
		return 0
	}
	var sum float64
	for _, n := range nums {
		sum += n
	}
	return sum / float64(len(nums))
}

// isDataRow — 날짜 형식으로 시작하는 데이터 행인지 확인
func isDataRow(line string) bool {

	if !strings.HasPrefix(line, "| ") {
		return false
	}

	// "|" 로 분리해서 첫 번째 컬럼 (날짜) 추출
	parts := strings.Split(line, "|")
	if len(parts) < 2 {
		return false
	}

	dateStr := strings.TrimSpace(parts[1])

	// 날짜 형식 최소 길이 확인 (2026-08-20 = 10글자)
	if len(dateStr) < 10 {
		return false
	}

	if dateStr[4] != '-' || dateStr[7] != '-' {
		return false
	}

	_, err := strconv.Atoi(dateStr[:4])
	return err == nil
}