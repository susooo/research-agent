package renderer

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"research-agent/models"
)

// SaveQuality — 검색 품질 점수 누적 저장
func SaveQuality(result models.QualityResult) error {
	path := "output/quality.md"

	existing := ""
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	}

	if existing == "" {
		existing = buildEmptyQuality()
	}

	// 새 행 추가
	newRow := fmt.Sprintf(
		"| %s | %s | %s | %.2f | %.2f | %.2f | %s |\n",
		result.RunAt[:10],
		result.Topic,
		result.Version,
		result.AvgScore,
		result.MaxScore,
		result.MinScore,
		result.Note,
	)

	updated := strings.Replace(
		existing,
		"<!-- quality row -->",
		newRow+"<!-- quality row -->",
		1,
	)

	// 버전별 평균 재계산
	updated = recalculateQuality(updated)

	return os.WriteFile(path, []byte(updated), 0644)
}

// buildEmptyQuality — 빈 파일 초기 구조
func buildEmptyQuality() string {
	return `# 📊 검색 품질 기록

> Tavily 검색 품질 점수 및 프롬프트 버전별 변화 추이

## 품질 기록

| 날짜 | 주제 | 버전 | 평균 점수 | 최고 점수 | 최저 점수 | 변경 내용 |
| --- | --- | --- | ---: | ---: | ---: | --- |
<!-- quality row -->

## 📊 버전별 평균

| 버전 | 평균 점수 | 변경 내용 | 측정 횟수 |
| --- | ---: | --- | ---: |
<!-- version avg -->
`
}

// recalculateQuality — 버전별 평균 재계산
func recalculateQuality(content string) string {
	lines := strings.Split(content, "\n")

	// 버전별 데이터 수집
	type versionData struct {
		scores []float64
		note   string
	}
	versionMap := map[string]*versionData{}
	versionOrder := []string{}

	inQualitySection := false
	for _, line := range lines {
		if strings.Contains(line, "## 품질 기록") {
			inQualitySection = true
			continue
		}
		if strings.Contains(line, "## 📊 버전별 평균") {
			inQualitySection = false
			continue
		}
		if !inQualitySection || !isDataRow(line) {
			continue
		}

		cols := strings.Split(line, "|")
		if len(cols) < 8 {
			continue
		}

		version := strings.TrimSpace(cols[3])
		avgStr := strings.TrimSpace(cols[4])
		note := strings.TrimSpace(cols[7])

		avg, err := strconv.ParseFloat(avgStr, 64)
		if err != nil {
			continue
		}

		if _, ok := versionMap[version]; !ok {
			versionMap[version] = &versionData{note: note}
			versionOrder = append(versionOrder, version)
		}
		versionMap[version].scores = append(versionMap[version].scores, avg)
		versionMap[version].note = note
	}

	// 버전별 평균 행 생성
	newRows := ""
	for _, v := range versionOrder {
		d := versionMap[v]
		avg := average(d.scores)
		newRows += fmt.Sprintf(
			"| %s | %.2f | %s | %d회 |\n",
			v,
			avg,
			d.note,
			len(d.scores),
		)
	}

	// version avg 교체
	inAvgSection := false
	headerPassed := false
	replaced := false
	result := ""

	for _, line := range lines {
		if strings.Contains(line, "## 📊 버전별 평균") {
			inAvgSection = true
			result += line + "\n"
			continue
		}

		if inAvgSection && !headerPassed {
			result += line + "\n"
			if strings.Contains(line, "| ---") {
				headerPassed = true
			}
			continue
		}

		if inAvgSection && headerPassed && !replaced {
			if strings.Contains(line, "<!-- version avg -->") {
				result += newRows
				result += line + "\n"
				replaced = true
				inAvgSection = false
				continue
			}
			continue
		}

		result += line + "\n"
	}

	return result
}