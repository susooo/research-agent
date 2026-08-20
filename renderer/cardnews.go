package renderer

import (
	"fmt"
	"html"
	"os"
	"strings"
	"time"

	"research-agent/agent"
	"research-agent/config"
)

// GenerateCardNews — HTML 카드뉴스 파일 생성
func GenerateCardNews(report *agent.Report, outputPath string, cfg *config.Config) error {
	if len(report.Cards) == 0 {
		return fmt.Errorf("카드 데이터가 없습니다")
	}

	htmlContent := buildHTML(report)
	return os.WriteFile(outputPath, []byte(htmlContent), 0644)
}

// buildHTML — 전체 HTML 조립
func buildHTML(report *agent.Report) string {
	now := time.Now().Format("2006년 01월 02일")

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="ko">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s — 리서치 카드뉴스</title>
<style>
  @import url('https://fonts.googleapis.com/css2?family=Noto+Sans+KR:wght@400;500;700;900&display=swap');

  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

  body {
    background: #0a0a0f;
    color: #f0f0f5;
    font-family: 'Noto Sans KR', sans-serif;
    min-height: 100vh;
  }

  /* ── 헤더 ── */
  .header {
    padding: 60px 40px 40px;
    max-width: 900px;
    margin: 0 auto;
    border-bottom: 1px solid rgba(255,255,255,0.08);
  }

  .header-meta {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 24px;
  }

  .badge {
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: #7c6bff;
    background: rgba(124,107,255,0.12);
    border: 1px solid rgba(124,107,255,0.3);
    padding: 4px 10px;
    border-radius: 4px;
  }

  .date {
    font-size: 12px;
    color: rgba(240,240,245,0.5);
  }

  .header h1 {
    font-size: clamp(28px, 5vw, 44px);
    font-weight: 900;
    line-height: 1.2;
    letter-spacing: -0.02em;
    margin-bottom: 12px;
  }

  .header h1 span { color: #7c6bff; }

  /* ── 카드 그리드 ── */
  .cards-section {
    max-width: 900px;
    margin: 0 auto;
    padding: 48px 40px;
  }

  .section-label {
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.15em;
    text-transform: uppercase;
    color: rgba(240,240,245,0.4);
    margin-bottom: 28px;
  }

  .cards-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: 16px;
  }

  /* ── 카드 ── */
  .card {
    background: #13131a;
    border: 1px solid rgba(255,255,255,0.08);
    border-radius: 16px;
    padding: 28px 24px;
    display: flex;
    flex-direction: column;
    gap: 16px;
    transition: transform 0.2s, border-color 0.2s, box-shadow 0.2s;
  }

  .card:hover {
    transform: translateY(-4px);
    border-color: rgba(124,107,255,0.4);
    box-shadow: 0 16px 48px rgba(124,107,255,0.12);
  }

  /* 첫 번째 카드는 전체 너비 */
  .card:first-child {
    grid-column: 1 / -1;
    flex-direction: row;
    align-items: flex-start;
    gap: 28px;
    background: linear-gradient(135deg, #13131a 0%%, #1a1630 100%%);
    border-color: rgba(124,107,255,0.25);
  }

  .card:first-child .card-emoji { font-size: 52px; flex-shrink: 0; }
  .card:first-child .card-headline { font-size: 22px; }

  .card-top {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 12px;
  }

  .card-emoji { font-size: 32px; line-height: 1; }

  .card-tag {
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    padding: 3px 8px;
    border-radius: 4px;
    flex-shrink: 0;
  }

  .tag-트렌드  { color: #7c6bff; background: rgba(124,107,255,0.12); border: 1px solid rgba(124,107,255,0.25); }
  .tag-핵심    { color: #ff6b9d; background: rgba(255,107,157,0.12); border: 1px solid rgba(255,107,157,0.25); }
  .tag-데이터  { color: #6bffce; background: rgba(107,255,206,0.12); border: 1px solid rgba(107,255,206,0.25); }
  .tag-전망    { color: #ffd166; background: rgba(255,209,102,0.12); border: 1px solid rgba(255,209,102,0.25); }
  .tag-사례    { color: #a8daff; background: rgba(168,218,255,0.12); border: 1px solid rgba(168,218,255,0.25); }
  .tag-인사이트 { color: #c8b4ff; background: rgba(200,180,255,0.12); border: 1px solid rgba(200,180,255,0.25); }

  .card-number {
    font-size: 11px;
    color: rgba(240,240,245,0.4);
    margin-top: 2px;
  }

  .card-headline {
    font-size: 17px;
    font-weight: 700;
    line-height: 1.3;
  }

  .card-body {
    font-size: 14px;
    color: rgba(240,240,245,0.7);
    line-height: 1.7;
    flex: 1;
  }

  .divider {
    height: 1px;
    background: rgba(255,255,255,0.08);
  }

  /* ── 출처 ── */
  .sources-section {
    max-width: 900px;
    margin: 0 auto;
    padding: 40px 40px 64px;
    border-top: 1px solid rgba(255,255,255,0.08);
  }

  .sources-section h3 {
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.15em;
    text-transform: uppercase;
    color: rgba(240,240,245,0.4);
    margin-bottom: 20px;
  }

  .source-item {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 8px;
  }

  .source-dot {
    width: 6px; height: 6px;
    border-radius: 50%%;
    background: #7c6bff;
    flex-shrink: 0;
  }

  .source-item a {
    font-size: 12px;
    color: rgba(240,240,245,0.5);
    text-decoration: none;
    word-break: break-all;
    transition: color 0.15s;
  }

  .source-item a:hover { color: #7c6bff; }

  /* ── 푸터 ── */
  .footer {
    text-align: center;
    padding: 24px;
    font-size: 11px;
    color: rgba(240,240,245,0.3);
    border-top: 1px solid rgba(255,255,255,0.08);
  }

  .footer span { color: #7c6bff; }

  @media (max-width: 600px) {
    .header, .cards-section, .sources-section { padding-left: 20px; padding-right: 20px; }
    .card:first-child { flex-direction: column; }
    .cards-grid { grid-template-columns: 1fr; }
  }
</style>
</head>
<body>

<header class="header">
  <div class="header-meta">
    <span class="badge">Research Agent</span>
    <span class="date">%s</span>
  </div>
  <h1>%s<br><span>핵심 인사이트</span></h1>
</header>

<section class="cards-section">
  <p class="section-label">// %d개 카드 · 실시간 웹 리서치</p>
  <div class="cards-grid">
    %s
  </div>
</section>

<section class="sources-section">
  <h3>// 참고 출처</h3>
  %s
</section>

<footer class="footer">
  Generated by <span>Research Agent</span> — Go + Gemini · %s
</footer>

</body>
</html>`,
		html.EscapeString(report.Topic),
		now,
		html.EscapeString(report.Topic),
		len(report.Cards),
		buildCards(report.Cards),
		buildSources(report.Sources),
		now,
	)
}

// buildCards — 카드 HTML 생성
func buildCards(cards []agent.CardContent) string {
	var sb strings.Builder
	total := len(cards)

	for _, c := range cards {
		tagClass := "tag-" + c.Tag
		sb.WriteString(fmt.Sprintf(`
    <article class="card">
      <div class="card-top">
        <span class="card-emoji">%s</span>
        <div style="display:flex;flex-direction:column;align-items:flex-end;gap:4px">
          <span class="card-tag %s">%s</span>
          <span class="card-number">%02d / %02d</span>
        </div>
      </div>
      <div class="divider"></div>
      <h2 class="card-headline">%s</h2>
      <p class="card-body">%s</p>
    </article>`,
			c.Emoji,
			tagClass,
			html.EscapeString(c.Tag),
			c.CardNumber,
			total,
			html.EscapeString(c.Headline),
			html.EscapeString(c.Body),
		))
	}
	return sb.String()
}

// buildSources — 출처 HTML 생성
func buildSources(sources []string) string {
	var sb strings.Builder
	for _, s := range sources {
		sb.WriteString(fmt.Sprintf(`
  <div class="source-item">
    <div class="source-dot"></div>
    <a href="%s" target="_blank" rel="noopener">%s</a>
  </div>`,
			html.EscapeString(s),
			html.EscapeString(s),
		))
	}
	return sb.String()
}