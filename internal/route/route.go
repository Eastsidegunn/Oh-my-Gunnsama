package route

import (
	"sort"
	"strings"

	"oh-my-gunnsama/internal/registry"
)

type RouteResult struct {
	Procedure    Procedure
	Domain       string
	Agent        string
	Confidence   Confidence
	Score        *float32
	Reasons      []string
	Triggers     []string
	RequiresUser bool
}

type Procedure string

const (
	ProcedureInterview Procedure = "interview"
	ProcedurePlan      Procedure = "plan"
	ProcedureExecute   Procedure = "execute"
	ProcedureReview    Procedure = "review"
	ProcedureVerify    Procedure = "verify"
	ProcedureCancel    Procedure = "cancel"
	ProcedureNone      Procedure = "none"
)

type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

type match struct {
	agent      registry.Agent
	trigger    registry.Trigger
	matched    []string
	priority   int
	matchCount int
	weak       bool
}

func Route(input string, reg registry.Registry) RouteResult {
	text := strings.ToLower(input)
	matches := findMatches(text, reg)
	if len(matches) == 0 {
		return RouteResult{
			Procedure:  ProcedureNone,
			Confidence: ConfidenceLow,
			Reasons:    []string{"no trigger match"},
		}
	}

	best := matches[0]
	confidence := confidenceFor(best)
	score := scoreFor(best)
	result := RouteResult{
		Procedure:  procedureFor(text),
		Domain:     best.trigger.Domain,
		Agent:      best.agent.Name,
		Confidence: confidence,
		Score:      &score,
		Reasons: []string{
			"matched registry trigger for agent " + best.agent.Name,
		},
		Triggers: best.matched,
	}
	if result.Procedure == ProcedureNone {
		result.Procedure = ProcedureExecute
	}
	if best.agent.Autonomy == "cautious" && confidence == ConfidenceLow {
		result.RequiresUser = true
	}
	return result
}

func findMatches(text string, reg registry.Registry) []match {
	matches := []match{}
	for _, name := range sortedAgentNames(reg.Agents) {
		agent := reg.Agents[name]
		for _, trigger := range agent.Triggers {
			terms := triggerTerms(trigger.Pattern)
			matched := matchedTerms(text, terms)
			if len(matched) == 0 {
				continue
			}
			matches = append(matches, match{
				agent:      agent,
				trigger:    trigger,
				matched:    matched,
				priority:   trigger.Priority,
				matchCount: len(matched),
				weak:       weakMatch(text, matched),
			})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].matchCount != matches[j].matchCount {
			return matches[i].matchCount > matches[j].matchCount
		}
		if matches[i].priority != matches[j].priority {
			return matches[i].priority > matches[j].priority
		}
		return matches[i].agent.Name < matches[j].agent.Name
	})
	return matches
}

func confidenceFor(m match) Confidence {
	if m.matchCount >= 2 {
		return ConfidenceHigh
	}
	if m.weak {
		return ConfidenceLow
	}
	return ConfidenceMedium
}

func scoreFor(m match) float32 {
	score := float32(m.matchCount)
	if m.priority > 0 {
		score += float32(m.priority) / 100
	}
	if m.weak {
		score *= 0.5
	}
	return score
}

func procedureFor(text string) Procedure {
	switch {
	case containsAny(text, []string{"cancel", "stop", "abort", "취소", "중단"}):
		return ProcedureCancel
	case containsAny(text, []string{"interview", "clarify", "question", "모호", "질문", "인터뷰"}):
		return ProcedureInterview
	case containsAny(text, []string{"plan", "roadmap", "spec", "계획", "설계"}):
		return ProcedurePlan
	case containsAny(text, []string{"review", "audit", "inspect", "검토", "리뷰"}):
		return ProcedureReview
	case containsAny(text, []string{"verify", "test", "validate", "검증", "테스트"}):
		return ProcedureVerify
	case containsAny(text, []string{"implement", "build", "create", "fix", "add", "구현", "수정", "추가"}):
		return ProcedureExecute
	default:
		return ProcedureNone
	}
}

func triggerTerms(pattern string) []string {
	parts := strings.FieldsFunc(strings.ToLower(pattern), func(r rune) bool {
		return r == '|' || r == ',' || r == ';' || r == '\n' || r == '\t'
	})
	terms := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			terms = append(terms, trimmed)
		}
	}
	return terms
}

func matchedTerms(text string, terms []string) []string {
	matched := []string{}
	for _, term := range terms {
		if strings.Contains(text, term) {
			matched = append(matched, term)
		}
	}
	return matched
}

func weakMatch(text string, matched []string) bool {
	if len(matched) != 1 {
		return false
	}
	term := matched[0]
	return strings.Contains(text, "maybe "+term) || strings.Contains(text, term+"?")
}

func containsAny(text string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func sortedAgentNames(agents map[string]registry.Agent) []string {
	names := make([]string, 0, len(agents))
	for name := range agents {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
