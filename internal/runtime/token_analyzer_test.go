package runtime

import "testing"

func TestTokenAnalyzerSummaryOmitsUnknownCost(t *testing.T) {
	analyzer := NewTokenAnalyzer("session")
	analyzer.Record(TokenMetrics{
		InputTokens:  10,
		OutputTokens: 5,
		TotalTokens:  15,
		Component:    "assistant",
		Stage:        "response",
	})

	summary := analyzer.GetSummary()
	if _, ok := summary["estimated_cost"]; ok {
		t.Fatalf("summary should omit estimated_cost when provider usage cost is unknown")
	}
}
