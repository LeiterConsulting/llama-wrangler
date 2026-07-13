package consensus

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
)

const (
	StrategyExactNormalized = "exact_normalized"
	StrategyJSONStructural  = "json_structural"
)

type Candidate struct {
	NodeID string
	Body   []byte
}

type Validator interface {
	Validate(surface string, candidate Candidate) bool
}

type Evaluator interface {
	Score(surface string, candidate Candidate) float64
}

type Engine struct {
	Validator Validator
	Evaluator Evaluator
}

type Result struct {
	Winner           Candidate
	AgreementScore   float64
	AgreementCount   int
	CandidateCount   int
	ConsensusReached bool
	Disagreement     bool
	Strategy         string
	ValidatorPassed  bool
	EvaluatorScore   float64
}

type NoopEvaluator struct{}

func (NoopEvaluator) Score(string, Candidate) float64 { return 0 }

type RegexValidator struct {
	pattern *regexp.Regexp
}

func NewRegexValidator(pattern string) (*RegexValidator, error) {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &RegexValidator{pattern: compiled}, nil
}

func (v *RegexValidator) Validate(surface string, candidate Candidate) bool {
	if v == nil || v.pattern == nil {
		return false
	}
	content, _ := responseContent(candidate.Body, surface)
	return v.pattern.MatchString(content)
}

type candidateScore struct {
	candidate       Candidate
	signature       string
	strategy        string
	validatorPassed bool
	evaluatorScore  float64
	index           int
}

type agreementGroup struct {
	members         []candidateScore
	firstIndex      int
	strategy        string
	validatorPassed bool
	evaluatorScore  float64
}

func (e Engine) Evaluate(surface string, candidates []Candidate) Result {
	result := Result{CandidateCount: len(candidates)}
	if len(candidates) == 0 {
		return result
	}
	evaluator := e.Evaluator
	if evaluator == nil {
		evaluator = NoopEvaluator{}
	}
	groups := map[string]*agreementGroup{}
	order := []string{}
	for index, candidate := range candidates {
		signature, strategy := responseSignature(candidate.Body, surface)
		scored := candidateScore{
			candidate:      candidate,
			signature:      signature,
			strategy:       strategy,
			evaluatorScore: evaluator.Score(surface, candidate),
			index:          index,
		}
		if e.Validator != nil {
			scored.validatorPassed = e.Validator.Validate(surface, candidate)
		}
		group, ok := groups[signature]
		if !ok {
			group = &agreementGroup{firstIndex: index, strategy: strategy}
			groups[signature] = group
			order = append(order, signature)
		}
		group.members = append(group.members, scored)
		if scored.validatorPassed {
			group.validatorPassed = true
		}
		if scored.evaluatorScore > group.evaluatorScore {
			group.evaluatorScore = scored.evaluatorScore
		}
	}

	winner := groups[order[0]]
	for _, signature := range order[1:] {
		candidate := groups[signature]
		if betterGroup(candidate, winner) {
			winner = candidate
		}
	}
	winnerMember := winner.members[0]
	for _, member := range winner.members[1:] {
		if member.validatorPassed && !winnerMember.validatorPassed {
			winnerMember = member
			continue
		}
		if member.validatorPassed == winnerMember.validatorPassed && member.evaluatorScore > winnerMember.evaluatorScore {
			winnerMember = member
		}
	}
	result.Winner = winnerMember.candidate
	result.AgreementCount = len(winner.members)
	result.AgreementScore = float64(result.AgreementCount) / float64(len(candidates))
	result.ConsensusReached = result.AgreementCount >= 2 && result.AgreementCount*2 > len(candidates)
	result.Disagreement = len(groups) > 1
	result.Strategy = winner.strategy
	result.ValidatorPassed = winnerMember.validatorPassed
	result.EvaluatorScore = winnerMember.evaluatorScore
	return result
}

func betterGroup(candidate, current *agreementGroup) bool {
	if len(candidate.members) != len(current.members) {
		return len(candidate.members) > len(current.members)
	}
	if candidate.validatorPassed != current.validatorPassed {
		return candidate.validatorPassed
	}
	if candidate.evaluatorScore != current.evaluatorScore {
		return candidate.evaluatorScore > current.evaluatorScore
	}
	return candidate.firstIndex < current.firstIndex
}

func responseSignature(body []byte, surface string) (string, string) {
	content, structured := responseContent(body, surface)
	if structured {
		var value interface{}
		decoder := json.NewDecoder(strings.NewReader(content))
		decoder.UseNumber()
		if decoder.Decode(&value) == nil {
			if canonical, err := json.Marshal(value); err == nil {
				return "json:" + string(canonical), StrategyJSONStructural
			}
		}
	}
	return "text:" + normalizeText(content), StrategyExactNormalized
}

func responseContent(body []byte, surface string) (string, bool) {
	var payload map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if decoder.Decode(&payload) != nil {
		return string(body), false
	}
	var value interface{}
	switch surface {
	case "openai_chat_completions":
		if choices, ok := payload["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				if message, ok := choice["message"].(map[string]interface{}); ok {
					value = message["content"]
				}
			}
		}
	case "openai_completions":
		if choices, ok := payload["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				value = choice["text"]
			}
		}
	case "ollama_chat":
		if message, ok := payload["message"].(map[string]interface{}); ok {
			value = message["content"]
		}
	case "ollama_generate":
		value = payload["response"]
	default:
		value = payload
	}
	if text, ok := value.(string); ok {
		trimmed := strings.TrimSpace(text)
		return trimmed, looksLikeJSON(trimmed)
	}
	if value != nil {
		if canonical, err := json.Marshal(value); err == nil {
			return string(canonical), true
		}
	}
	if canonical, err := json.Marshal(payload); err == nil {
		return string(canonical), true
	}
	return string(body), false
}

func looksLikeJSON(value string) bool {
	return (strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}")) ||
		(strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]"))
}

func normalizeText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}
