package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"research-agent/agent"
	"research-agent/config"
	"research-agent/renderer"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorGray   = "\033[90m"
)

func main() {
	// 설정 로드 (API 키 확인)
	cfg := config.Load()

	// 터미널 입력 읽기
	scanner := bufio.NewScanner(os.Stdin)

	printBanner()

	// ── Step 1: 주제 입력 ──
	fmt.Print(colorCyan + "\n🔍 리서치할 주제를 입력하세요: " + colorReset)
	scanner.Scan()
	topic := strings.TrimSpace(scanner.Text())
	if topic == "" {
		fmt.Println(colorRed + "❌ 주제를 입력해주세요." + colorReset)
		os.Exit(1)
	}

	// ── Step 2: 리서치 실행 ──
	fmt.Println()
	fmt.Println(colorYellow + "⚡ 병렬 웹 리서치 시작..." + colorReset)
	fmt.Println(strings.Repeat("─", 50))

	a := agent.New(cfg)
	report, err := a.Research(topic)
	if err != nil {
		fmt.Println(colorRed+"❌ 리서치 실패:"+colorReset, err)
		os.Exit(1)
	}

	// ── Step 3: 리포트 저장 ──
	if err := os.MkdirAll("output", 0755); err != nil {
		fmt.Println(colorRed+"❌ output 폴더 생성 실패:"+colorReset, err)
		os.Exit(1)
	}

	timestamp := time.Now().Format("20060102_150405")
	reportPath := fmt.Sprintf("output/report_%s_%s.md", topic, timestamp)

	if err := os.WriteFile(reportPath, []byte(report.Markdown), 0644); err != nil {
		fmt.Println(colorRed+"❌ 리포트 저장 실패:"+colorReset, err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println(colorGreen + colorBold + "✅ 리포트 생성 완료!" + colorReset)
	fmt.Println("📄 저장 위치: " + reportPath)
	fmt.Println(strings.Repeat("─", 50))

	// ── Step 4: 리포트 미리보기 ──
	fmt.Println(colorBold + "\n📋 리포트 미리보기:" + colorReset)
	fmt.Println()
	printPreview(report.Markdown)
	fmt.Println(strings.Repeat("─", 50))

	// ── Step 5: Human-in-the-loop (승인 게이트) ──
	fmt.Println()
	fmt.Print(colorYellow + colorBold + "🤔 카드뉴스를 생성할까요? [y/n]: " + colorReset)
	scanner.Scan()
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))

	if answer != "y" && answer != "yes" {
		fmt.Println()
		fmt.Println(colorCyan + "👋 리포트만 저장했습니다!" + colorReset)
		fmt.Println("   위치: " + reportPath)
		return
	}

	// ── Step 6: 카드뉴스 생성 ──
	fmt.Println()
	fmt.Println(colorYellow + "🎨 카드뉴스 생성 중..." + colorReset)

	cardPath := fmt.Sprintf("output/cards_%s_%s.html", topic, timestamp)
	if err := renderer.GenerateCardNews(report, cardPath, cfg); err != nil {
		fmt.Println(colorRed+"❌ 카드뉴스 생성 실패:"+colorReset, err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println(colorGreen + colorBold + "✅ 카드뉴스 생성 완료!" + colorReset)
	fmt.Println("🃏 저장 위치: " + cardPath)
	fmt.Println()
	fmt.Println(colorCyan + "💡 브라우저에서 열어서 확인하세요!" + colorReset)
}

// printBanner — 시작 화면
func printBanner() {
	fmt.Println(colorCyan + colorBold + `
══════════════════════════════════════════
          🔍  Research Agent
     AI 리서치 자동화 · Go + Gemini
  웹 검색 → 리포트 → 카드뉴스 자동 생성
══════════════════════════════════════════` + colorReset)
	fmt.Println()
}

// printPreview — 리포트 앞부분 20줄 출력
func printPreview(markdown string) {
	lines := strings.Split(markdown, "\n")
	limit := 20
	if len(lines) < limit {
		limit = len(lines)
	}
	for _, line := range lines[:limit] {
		fmt.Println(line)
	}
	if len(lines) > 20 {
		fmt.Printf(colorGray+"\n... 외 %d줄 (전체는 파일에서 확인)"+colorReset+"\n",
			len(lines)-20)
	}
}