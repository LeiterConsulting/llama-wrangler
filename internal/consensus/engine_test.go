package consensus

import (
	"fmt"
	"testing"
)

func TestEvaluateNormalizedMajoritySelectsRankedWinner(t *testing.T) {
	result := (Engine{}).Evaluate("openai_chat_completions", []Candidate{
		{NodeID: "rank-one", Body: openAIChatBody("The answer is 42.")},
		{NodeID: "rank-two", Body: openAIChatBody("  the   answer is 42. ")},
		{NodeID: "rank-three", Body: openAIChatBody("A different answer.")},
	})
	if result.Winner.NodeID != "rank-one" || !result.ConsensusReached || !result.Disagreement {
		t.Fatalf("normalized majority result = %#v", result)
	}
	if result.AgreementCount != 2 || result.AgreementScore < 0.66 || result.Strategy != StrategyExactNormalized {
		t.Fatalf("normalized agreement = %#v", result)
	}
}

func TestEvaluateJSONStructuralAgreementIgnoresObjectKeyOrder(t *testing.T) {
	result := (Engine{}).Evaluate("ollama_generate", []Candidate{
		{NodeID: "json-one", Body: ollamaGenerateBody(`{"answer":42,"ok":true}`)},
		{NodeID: "json-two", Body: ollamaGenerateBody(`{"ok":true,"answer":42}`)},
	})
	if result.Winner.NodeID != "json-one" || !result.ConsensusReached || result.Strategy != StrategyJSONStructural || result.AgreementScore != 1 {
		t.Fatalf("JSON structural result = %#v", result)
	}
}

func TestEvaluateNoMajorityUsesRankOrderAndValidatorHook(t *testing.T) {
	validator, err := NewRegexValidator(`approved`)
	if err != nil {
		t.Fatalf("validator: %v", err)
	}
	result := (Engine{Validator: validator}).Evaluate("openai_completions", []Candidate{
		{NodeID: "rank-one", Body: openAICompletionBody("ordinary")},
		{NodeID: "rank-two", Body: openAICompletionBody("approved result")},
	})
	if result.Winner.NodeID != "rank-two" || result.ConsensusReached || !result.ValidatorPassed || result.AgreementScore != 0.5 {
		t.Fatalf("validator tie-break result = %#v", result)
	}

	plain := (Engine{}).Evaluate("openai_completions", []Candidate{
		{NodeID: "rank-one", Body: openAICompletionBody("first")},
		{NodeID: "rank-two", Body: openAICompletionBody("second")},
	})
	if plain.Winner.NodeID != "rank-one" || plain.ConsensusReached || !plain.Disagreement {
		t.Fatalf("rank tie-break result = %#v", plain)
	}
}

func openAIChatBody(content string) []byte {
	return []byte(fmt.Sprintf(`{"id":"response","choices":[{"message":{"role":"assistant","content":%q}}]}`, content))
}

func openAICompletionBody(content string) []byte {
	return []byte(fmt.Sprintf(`{"id":"response","choices":[{"text":%q}]}`, content))
}

func ollamaGenerateBody(content string) []byte {
	return []byte(fmt.Sprintf(`{"model":"test","response":%q,"done":true}`, content))
}
