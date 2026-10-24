package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"research-agent/models"
)

// plan — 주제 분석 후 초기 계획 수립
func (a *Agent) plan(ctx context.Context, topic string) (models.ResearchPlan, error) {
	prompt := fmt.Sprintf(`당신은 리서치 Agent의 Planner입니다.
사용자의 주제를 분석하고, 조사 목표를 달성하기 위한 초기 검색 계획을 수립하세요.
처음에는 가장 적합한 Tool 하나만 선택합니다. 이후 검색 결과 평가를 통해 추가 검색 여부와 다음 Tool을 결정합니다.

=== 주제 ===
%s

=== 사용 가능한 Tool ===

1. tavily — 최신 웹 정보 수집

주요 대상: 최신 뉴스, 기업 공식 블로그, 기술 문서, 시장 보고서, 통계 자료
사용 목적: 최신 기술 동향, 산업 현황, 기업 도입 사례 및 시장 규모 조사
우선 선택 조건: 최신성, 시의성, 실제 산업 적용 사례가 중요한 경우

2. exa — 의미 기반 탐색 및 관련 사례 발굴

주요 대상: 질문의 의미와 맥락에 관련된 웹페이지, 기술 자료, 유사 프로젝트 및 구현 사례
사용 목적: 새로운 접근법 탐색, 유사 기술 발굴, 관련 프로젝트 조사, 기존 검색에서 놓친 자료 보완
우선 선택 조건: 정확한 키워드 검색보다 의미적으로 연관된 자료와 새로운 관점을 발견하는 것이 중요한 경우
주의: 논문 검색도 가능하지만, 학술 논문 메타데이터와 인용 관계를 체계적으로 확보하는 것이 주목적이라면 scholar를 우선합니다.

3. scholar — 학술 논문 및 연구 근거 확보

주요 대상: 학술 논문, 초록, 저자, 출판 연도, 인용 수, 참고 문헌 및 인용 관계
사용 목적: 관련 논문 수집, 연구 결과 비교, 주요 연구 주제 분류, 학술적 근거 확보
우선 선택 조건: 논문 자체가 조사 대상이거나, 연구 결과와 학술적 근거를 체계적으로 분석해야 하는 경우
주의: 인용 수가 높다는 이유만으로 논문의 품질이나 신뢰성을 단정하지 않습니다. 최신 논문은 인용 수가 낮을 수 있으므로 출판 연도와 연구 내용도 고려합니다.

=== Tool 선택 기준 ===

- 최신 동향, 뉴스, 시장 현황, 기업 도입 사례가 핵심이면 tavily를 우선합니다.
- 의미적으로 관련된 기술, 유사 프로젝트, 새로운 접근법을 탐색하는 것이 핵심이면 exa를 우선합니다.
- 학술 논문 수집, 연구 결과 비교, 인용 및 참고 문헌 분석이 핵심이면 scholar를 우선합니다.
- 복합 주제라도 처음부터 여러 Tool을 선택하지 않습니다. 가장 중요한 조사 목적에 적합한 Tool 하나를 선택합니다.
- 주제에 "연구" 또는 "기술"이라는 단어가 있다는 이유만으로 scholar나 exa를 자동 선택하지 않습니다. 실제로 필요한 결과물이 무엇인지 판단합니다.
- 통계나 수치가 필요하면 tavily를 우선하되, 수치의 출처와 기준 시점을 확인할 수 있도록 검색 계획을 수립합니다.

=== 검색 쿼리 생성 규칙 ===

- 최소 3개, 최대 5개의 검색 쿼리를 생성합니다.
- 영어/한국어 혼용 가능
- 각 쿼리는 주제의 서로 다른 관점을 다루어야 합니다.
	예를 들어 시장 동향, 핵심 기술, 기업 적용 사례, 성과, 향후 전망 등을 고려합니다.
- 주제와 조사 목적에 적합한 관점을 선택하며, 모든 관점을 억지로 포함하지 않습니다.
- 단순히 표현만 바꾼 유사한 쿼리는 생성하지 않습니다. 각 쿼리가 서로 다른 정보를 확보하도록 작성합니다.
- 선택한 Tool의 특성과 조사 목표를 고려해 구체적인 검색 쿼리를 작성합니다.

=== 조사 목표 설정 ===

- min_sources: 확보해야 하는 최소 유효 출처 수
- min_score: 검색 결과의 최소 품질 점수
- need_stats: 수치나 통계가 필요한지 여부
- need_papers: 학술 논문이 필요한지 여부
- need_recent: 최신 자료가 필요한지 여부

주제의 성격에 따라 조사 목표를 설정합니다. 모든 주제에 통계와 최신 자료가 반드시 필요한 것은 아닙니다.
각 항목은 주제와 조사 목적에 따라 독립적으로 판단합니다.

1. need_stats
시장 규모, 성능 비교, 비용, 정량적 성과 등을 분석해야 한다면 true로 설정합니다.
정성적인 개념 설명이나 사례 분석만으로 충분하다면 false로 설정할 수 있습니다.

2. need_papers
알고리즘의 성능 비교, 실험 결과 분석, 연구 방법론 비교, 학술적 근거 확보가 핵심이라면 true로 설정합니다.
기업 도입 현황, 최신 산업 동향, 제품 비교 등이 핵심이라면 논문이 필수적이지 않을 수 있으므로 false로 설정합니다.
주제에 'AI', '기술', '연구'라는 단어가 포함되었다는 이유만으로 true로 설정하지 않습니다.

3. need_recent
최신 기술 동향, 현재 시장 상황, 최근 기업 도입 사례 등이 중요하다면 true로 설정합니다.
변하지 않는 기초 개념이나 역사적 원리를 설명하는 것이 목적이라면 false로 설정할 수 있습니다.

=== 출력 형식 (JSON만) ===
{
  "steps": [
    {
      "tool": "tavily 또는 exa 또는 scholar",
      "queries": ["검색 쿼리 1", "검색 쿼리 2", "검색 쿼리 3"],
      "purpose": "이 Tool을 첫 번째로 선택한 이유"
    }
  ],
  "goal": {
    "min_sources": 12,
    "min_score": 0.8,
    "need_stats": true 또는 false,
    "need_papers": true 또는 false,
    "need_recent": true 또는 false
  }
}

=== 필수 규칙 ===

- steps에는 반드시 하나의 Tool만 작성합니다.
- tool 값은 tavily, exa, scholar 중 하나여야 합니다.
- queries의 길이는 반드시 3 이상 5 이하여야 합니다.
- 각 쿼리는 서로 다른 조사 관점을 담당해야 합니다.
- JSON 이외의 설명이나 마크다운을 출력하지 않습니다.
- goal 값은 주제에 맞게 설정합니다.
- purpose에는 해당 Tool을 선택한 구체적인 이유를 작성합니다.`, topic)

	resp, err := a.callGemini(ctx, prompt)
	if err != nil {
		return models.ResearchPlan{}, err
	}

	resp = cleanJSON(resp)

	var plan models.ResearchPlan
	if err := json.Unmarshal([]byte(resp), &plan); err != nil {
		return models.ResearchPlan{}, fmt.Errorf("Plan 파싱 실패: %w", err)
	}

	return plan, nil
}

// decide — 현재 상태 보고 다음 행동 결정
func (a *Agent) decide(ctx context.Context, topic string, eval models.EvalResult, goal models.PlanGoal, usedTools []string, loopCount int,) (models.AgentAction, error) {

	// 최대 검색 횟수 도달
	if loopCount >= 3 {
		return models.AgentAction{
			Action: "finish",
			Reason: "최대 검색 횟수에 도달하여 현재 결과로 보고서를 작성합니다.",
		}, nil
	}

	// 조사 목표 달성
    if eval.GoalMet {
        return models.AgentAction{
            Action: "finish",
            Reason: "모든 조사 목표를 충족했습니다.",
        }, nil
    }

    // 사용한 Tool 목록
    usedSet := make(map[string]bool)
    for _, tool := range usedTools {
        usedSet[strings.ToLower(strings.TrimSpace(tool))] = true
    }

    usedStr := "없음"
    if len(usedTools) > 0 {
        usedStr = strings.Join(usedTools, ", ")
    }

    missingStr := strings.Join(eval.Missing, ", ")
    if missingStr == "" {
        missingStr = "명확하게 분류되지 않은 목표 미충족"
    }

    prompt := fmt.Sprintf(`당신은 리서치 Agent의 Action Selector입니다.

현재 검색 결과와 부족한 정보를 분석하여 다음 행동을 하나 선택하세요.

=== 주제 ===
%s

=== 현재 검색 결과 ===
- 출처 수: %d개
- 평균 품질 점수: %.2f
- 최신 자료 수: %d개
- 통계 포함 여부: %v
- 논문 포함 여부: %v
- 부족한 항목: %s

=== 이미 사용한 Tool ===
%s

=== 사용 가능한 Tool과 역할 ===

1. scholar: 학술 논문, 연구 결과, 인용 및 참고 문헌 검색
2. exa: 관련 기술, 유사 프로젝트, 새로운 접근법 및 구현 사례 탐색
3. tavily: 최신 동향, 뉴스, 공식 자료, 통계 및 기업 도입 사례 검색
4. finish: 추가 검색을 종료하고 현재 결과로 보고서 작성

=== 판단 규칙 ===

1. Tool 선택
- papers → scholar, recent/stats → tavily, examples → exa를 우선 고려합니다.
- 여러 항목이 부족하면 주제의 핵심 목적과 한 번의 검색으로 보완할 수 있는 정보를 고려해 선택합니다.
- sources/quality는 부족한 정보의 유형과 기존 결과의 품질을 고려해 판단합니다.

2. Tool 재사용
- 적합한 미사용 Tool을 우선 고려하되, 관련성이 낮다면 선택하지 않습니다.
- 이미 사용한 Tool도 부족한 정보를 더 효과적으로 확보할 수 있다면 재사용할 수 있습니다.
- 재사용 시 기존 검색과 다른 관점이나 세부 주제를 조사합니다. 모든 Tool을 사용할 필요는 없습니다.

3. 검색 쿼리 생성
- 검색을 선택하면 2~3개의 구체적인 쿼리를 생성합니다.
- 각 쿼리는 부족한 정보를 보완하는 서로 다른 관점을 다루어야 합니다.
- 기존 쿼리와 검색 의도가 겹치지 않도록 하며, Tool의 특성에 맞게 작성합니다.
- 최신 자료는 기준 기간을, 통계는 수치·출처·기준 연도를, 논문은 연구 주제·방법·결과를 고려합니다.
- 관련성 높은 쿼리를 만들기 어렵다면 finish를 선택합니다.

4. 종료 조건
- 조사 목표를 달성했거나 추가 검색의 기대 효과가 낮다면 finish를 선택합니다.
- 목표가 미충족이어도 적절한 추가 검색 방법이 없다면 종료 이유를 명시합니다.
- 최대 반복 횟수에 도달하면 추가 검색을 하지 않습니다.

=== 출력 형식 ===

JSON만 반환합니다.

{
  "action": "tavily 또는 exa 또는 scholar 또는 finish",
  "queries": ["검색 쿼리 1", "검색 쿼리 2"],
  "reason": "현재 부족한 항목과 선택한 행동의 이유"
}

=== 필수 규칙 ===
- action은 tavily, exa, scholar, finish 중 하나여야 합니다.
- finish를 선택하면 queries는 빈 배열로 반환합니다.
- JSON 외의 설명을 출력하지 않습니다.
`,
        topic,
        eval.SourceCount,
        eval.AvgScore,
        eval.RecentCount,
        eval.HasStats,
        eval.HasPapers,
        missingStr,
        usedStr,
    )

    resp, err := a.callGemini(ctx, prompt)
    if err != nil {
        return models.AgentAction{
            Action: "finish",
            Reason: "다음 행동 판단 실패: " + err.Error(),
        }, nil
    }

    resp = cleanJSON(resp)

    var action models.AgentAction
    if err := json.Unmarshal([]byte(resp), &action); err != nil {
        return models.AgentAction{
            Action: "finish",
            Reason: "다음 행동 응답 파싱 실패",
        }, nil
    }

    // 허용된 Action인지 검증
    switch action.Action {
    case "tavily", "exa", "scholar", "finish":
    default:
        return models.AgentAction{
            Action: "finish",
            Reason: "허용되지 않은 Action이 반환되어 종료합니다.",
        }, nil
    }

    // 검색을 선택했지만 쿼리가 없는 경우
    if action.Action != "finish" && len(action.Queries) == 0 {
        return models.AgentAction{
            Action: "finish",
            Reason: "검색 쿼리가 없어 추가 검색을 진행할 수 없습니다.",
        }, nil
    }

    return action, nil
}

func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}