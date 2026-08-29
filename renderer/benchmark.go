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
		"<!-- parallel row -->",
		newRow+"<!-- parallel row -->",
		1,
	)

	// 평균 재계산
	updated = recalculateAverage(updated)

	return os.WriteFile(path, []byte(updated), 0644)
}

// buildEmptyBenchmark — 빈 파일 초기 구조
func buildEmptyBenchmark() string {
	return `# 📈 벤치마크 기록

> Research Agent의 검색 실행 방식과 병렬 처리 성능을 측정한 기록


## 1. 순차 vs 병렬 실행

| 날짜 | 주제 | 순차 실행 | 병렬 실행 | 개선율 | 단축률 |
| --- | --- | --- | --- | --- | --- |
<!-- parallel row -->


## 📊 순차 vs 병렬 평균

| 평균 순차 실행 | 평균 병렬 실행 | 평균 개선율 | 평균 단축률 | 총 측정 횟수 |
| --- | --- | --- | --- | --- |
| - | - | - | - | 0회 |

`
}

// SaveTradeoffBenchmark — 트레이드오프 누적 저장
func SaveTradeoffBenchmark(result models.TradeoffBenchmarkResult) error {
	path := "output/benchmark.md"

	existing := ""
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	}
	if existing == "" {
		existing = buildEmptyBenchmark()
	}

	// 섹션 3 없으면 추가
	if !strings.Contains(existing, "## 2. Worker 수별 트레이드오프") {
		existing += buildEmptyTradeoffSection()
	}

	// 주제별 표 생성
	rows := fmt.Sprintf("\n### 검색 주제: %s (%s)\n\n", result.Topic, result.RunAt[:10])
	rows += "| Worker 수 | 실행 시간 | 순차 대비 | 메모리 | 에러 |\n"
	rows += "| ---: | ---: | ---: | ---: | ---: |\n"

	// 순차 시간 찾기 (기준값)
	var seqTime float64
	for _, r := range result.Results {
		if r.Workers == 1 {
			seqTime = float64(r.ElapsedMs)
			break
		}
	}

	for _, r := range result.Results {
		workerStr := fmt.Sprintf("%d", r.Workers)
		if r.Workers == 1 {
			workerStr = "1 (순차)"
		}

		seqRatio := "기준"
		if r.Workers != 1 && seqTime > 0 {
			seqRatio = fmt.Sprintf("%.1f배", seqTime/float64(r.ElapsedMs))
		}

		errorStr := "-"
		if r.Failed {
			errorStr = "❌"
		}

		rows += fmt.Sprintf(
			"| %s | %dms | %s | %.2fMB | %s |\n",
			workerStr,
			r.ElapsedMs,
			seqRatio,
			r.MemoryMB,
			errorStr,
		)
	}

	// 주제별 표 추가
	updated := strings.Replace(
		existing,
		"<!-- tradeoff row -->",
		rows+"<!-- tradeoff row -->",
		1,
	)

	// 누적 에러율 재계산
	updated = recalculateErrorRate(updated, result)

	return os.WriteFile(path, []byte(updated), 0644)
}

// buildEmptyTradeoffSection — 모드 3 초기 구조
func buildEmptyTradeoffSection() string {
	return `

---

## 2. Worker 수별 트레이드오프

> Worker 수 증가에 따른 실행 시간, 메모리, 안정성 비교
> 실행 시간/메모리는 주제별, 에러율은 전체 누적

<!-- tradeoff row -->


## 📊 누적 에러율 (전체 실행 기준)

| Worker 수 | 총 실행 | 에러 횟수 | 에러율 | 안정성 |
| ---: | ---: | ---: | ---: | --- |
| 1 (순차) | 0회 | 0회 | 0% | ✅ 안정 |
| 2 | 0회 | 0회 | 0% | ✅ 안정 |
| 3 | 0회 | 0회 | 0% | ✅ 안정 |
| 4 | 0회 | 0회 | 0% | ✅ 안정 |
| 6 | 0회 | 0회 | 0% | ✅ 안정 |
<!-- error rate row -->

`
}

// recalculateErrorRate — 누적 에러율 재계산
func recalculateErrorRate(content string, result models.TradeoffBenchmarkResult) string {
	lines := strings.Split(content, "\n")

	// 워커별 에러 데이터 수집
	type errorData struct {
		total  int
		failed int
	}
	errorMap := map[int]*errorData{
		1: {}, 2: {}, 3: {}, 4: {}, 6: {},
	}

	// ==============================
	// 1. 기존 누적 에러율 읽기
	// ==============================
	inErrorSection := false

	for _, line := range lines {
		if strings.Contains(line, "## 📊 누적 에러율") {
			inErrorSection = true
			continue
		}
		if strings.Contains(line, "<!-- error rate row -->") {
			inErrorSection = false
			continue
		}
		if !inErrorSection {
			continue
		}

		trimmed := strings.TrimSpace(line)

		if !strings.HasPrefix(trimmed, "|") {
			continue
		}

		if strings.Contains(trimmed, "Worker 수") ||
			strings.Contains(trimmed, "---") {
			continue
		}

		cols := strings.Split(line, "|")
		if len(cols) < 7 {
			continue
		}

		// Worker 수 파싱
		workerStr := strings.TrimSpace(cols[1])
		workerStr = strings.Replace(workerStr, " (순차)", "", 1)
		workerNum, err := strconv.Atoi(workerStr)
		if err != nil {
			continue
		}

		d, ok := errorMap[workerNum]
		if !ok {
			continue
		}


		// 기존 총 실행, 에러 횟수 파싱
		totalStr := strings.TrimSuffix(strings.TrimSpace(cols[2]), "회")
		failStr := strings.TrimSuffix(strings.TrimSpace(cols[3]), "회")
		total, err1 := strconv.Atoi(totalStr)
		failed, err2 := strconv.Atoi(failStr)

		if err1 != nil || err2 != nil {
			continue
		}

		d.total = total
		d.failed = failed
	}

	// ==============================
	// 2. 이번 실행 결과 누적
	// ==============================
	for _, r := range result.Results {
		d, ok := errorMap[r.Workers]
		if !ok {
			continue
		}

		d.total++

		if r.Failed {
			d.failed++
		}
	}


	// ==============================
	// 3. 새로운 에러율 행 생성
	// ==============================
	workerOrder := []int{1, 2, 3, 4, 6}
	var newRows strings.Builder

	for _, w := range workerOrder {
		d := errorMap[w]

		workerStr := fmt.Sprintf("%d", w)

		if w == 1 {
			workerStr = "1 (순차)"
		}

		errorRate := 0.0

		if d.total > 0 {
			errorRate = float64(d.failed) /
				float64(d.total) * 100
		}

		stability := "✅ 안정"

		if errorRate > 0 && errorRate < 10 {
			stability = "⚠️ 주의"
		} else if errorRate >= 10 {
			stability = "❌ 불안정"
		}

		fmt.Fprintf(
			&newRows,
			"| %s | %d회 | %d회 | %.0f%% | %s |\n",
			workerStr,
			d.total,
			d.failed,
			errorRate,
			stability,
		)
	}

	// ==============================
	// 4. 기존 에러율 행 교체
	// ==============================
	var resultBuilder strings.Builder

	inSection := false
	headerPassed := false
	replaced := false

	for _, line := range lines {
	
		if strings.Contains(line, "## 📊 누적 에러율") {
			inSection = true
			resultBuilder.WriteString(line + "\n")
			continue
		}

		// 에러율 섹션 밖
		if !inSection {
			resultBuilder.WriteString(line + "\n")
			continue
		}

		// 헤더와 구분선 유지
		if !headerPassed {
			resultBuilder.WriteString(line + "\n")

			if strings.Contains(line, "| ---:") {
				headerPassed = true
			}

			continue
		}

		// 기존 데이터 행 제거 후 새로운 데이터 삽입
		if strings.Contains(line, "<!-- error rate row -->") {
			resultBuilder.WriteString(newRows.String())
			resultBuilder.WriteString(line + "\n")

			replaced = true
			inSection = false

			continue
		}
	}

	// marker를 찾지 못했다면 원본 반환
	if !replaced {
		return content
	}

	return resultBuilder.String()
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
		if strings.Contains(line, "## 📊 순차 vs 병렬 평균") {
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