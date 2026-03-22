package council

import (
	"reflect"
	"testing"
)

func TestParseRankingFromText(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "standard numbered FINAL RANKING section",
			text: "Response A is good.\nFINAL RANKING:\n1. Response A\n2. Response B\n3. Response C",
			want: []string{"Response A", "Response B", "Response C"},
		},
		{
			name: "reversed order in FINAL RANKING",
			text: "FINAL RANKING:\n1. Response C\n2. Response B\n3. Response A",
			want: []string{"Response C", "Response B", "Response A"},
		},
		{
			name: "unnumbered labels in FINAL RANKING section",
			text: "FINAL RANKING:\nResponse B\nResponse A",
			want: []string{"Response B", "Response A"},
		},
		{
			name: "labels scattered in body when no FINAL RANKING marker",
			text: "Response A is the best. Response C is mediocre. Response B is worst.",
			want: []string{"Response A", "Response C", "Response B"},
		},
		{
			name: "FINAL RANKING present but numbered list wins over body labels",
			text: "I prefer Response C.\nFINAL RANKING:\n1. Response A\n2. Response C",
			want: []string{"Response A", "Response C"},
		},
		{
			name: "garbled output with no response labels",
			text: "I cannot rank these responses.",
			want: nil,
		},
		{
			name: "empty text",
			text: "",
			want: nil,
		},
		{
			name: "label in body only — no FINAL RANKING",
			text: "Only one label: Response D mentioned here.",
			want: []string{"Response D"},
		},
		{
			name: "FINAL RANKING present but empty section — does not fall back to body",
			// Once the marker is found the function only searches within that section.
			// Labels in the body above the marker are intentionally ignored.
			text: "Response A was good. Response B was bad.\nFINAL RANKING:\nNo labels here.",
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseRankingFromText(tc.text)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseRankingFromText(%q)\n  got:  %v\n  want: %v", tc.text, got, tc.want)
			}
		})
	}
}

func TestCalculateAggregateRankings(t *testing.T) {
	t.Run("single ranker, three models", func(t *testing.T) {
		stage2 := []StageTwoResult{
			{Model: "modelX", ParsedRanking: []string{"Response A", "Response B", "Response C"}},
		}
		labelToModel := map[string]string{
			"Response A": "alpha",
			"Response B": "beta",
			"Response C": "gamma",
		}
		got, _ := CalculateAggregateRankings(stage2, labelToModel)
		if len(got) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(got))
		}
		// alpha is ranked 1st, beta 2nd, gamma 3rd — sort order should be ascending average rank
		if got[0].Model != "alpha" || got[0].AverageRank != 1.0 {
			t.Errorf("expected alpha at rank 1.0, got %+v", got[0])
		}
		if got[1].Model != "beta" || got[1].AverageRank != 2.0 {
			t.Errorf("expected beta at rank 2.0, got %+v", got[1])
		}
		if got[2].Model != "gamma" || got[2].AverageRank != 3.0 {
			t.Errorf("expected gamma at rank 3.0, got %+v", got[2])
		}
	})

	t.Run("multiple rankers, unanimous agreement", func(t *testing.T) {
		stage2 := []StageTwoResult{
			{Model: "m1", ParsedRanking: []string{"Response A", "Response B"}},
			{Model: "m2", ParsedRanking: []string{"Response A", "Response B"}},
		}
		labelToModel := map[string]string{"Response A": "alpha", "Response B": "beta"}
		got, _ := CalculateAggregateRankings(stage2, labelToModel)
		if len(got) != 2 {
			t.Fatalf("expected 2 aggregate rankings, got %d", len(got))
		}
		if got[0].Model != "alpha" || got[0].AverageRank != 1.0 || got[0].RankingsCount != 2 {
			t.Errorf("unexpected first entry: %+v", got[0])
		}
		if got[1].Model != "beta" || got[1].AverageRank != 2.0 || got[1].RankingsCount != 2 {
			t.Errorf("unexpected second entry: %+v", got[1])
		}
	})

	t.Run("multiple rankers, disagreement averages positions", func(t *testing.T) {
		// m1: A=1, B=2  →  m2: B=1, A=2  →  average: A=1.5, B=1.5
		stage2 := []StageTwoResult{
			{Model: "m1", ParsedRanking: []string{"Response A", "Response B"}},
			{Model: "m2", ParsedRanking: []string{"Response B", "Response A"}},
		}
		labelToModel := map[string]string{"Response A": "alpha", "Response B": "beta"}
		got, _ := CalculateAggregateRankings(stage2, labelToModel)
		if len(got) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(got))
		}
		for _, r := range got {
			if r.AverageRank != 1.5 {
				t.Errorf("expected average rank 1.5 for %s, got %v", r.Model, r.AverageRank)
			}
			if r.RankingsCount != 2 {
				t.Errorf("expected 2 rankings for %s, got %d", r.Model, r.RankingsCount)
			}
		}
		// Verify both expected models are present (order-independent — tied average ranks).
		expectedModels := map[string]bool{"alpha": true, "beta": true}
		actualModels := make(map[string]bool, len(got))
		for _, r := range got {
			actualModels[r.Model] = true
		}
		if !reflect.DeepEqual(expectedModels, actualModels) {
			t.Errorf("expected models %v, got %v", expectedModels, actualModels)
		}
	})

	t.Run("label not in labelToModel is silently skipped", func(t *testing.T) {
		stage2 := []StageTwoResult{
			{Model: "m1", ParsedRanking: []string{"Response Z", "Response A"}},
		}
		labelToModel := map[string]string{"Response A": "alpha"} // Response Z unmapped
		got, _ := CalculateAggregateRankings(stage2, labelToModel)
		if len(got) != 1 {
			t.Fatalf("expected 1 entry (unmapped label skipped), got %d", len(got))
		}
		// Response A is in position 2 (index 1 + 1)
		if got[0].Model != "alpha" || got[0].AverageRank != 2.0 {
			t.Errorf("unexpected entry: %+v", got[0])
		}
	})

	t.Run("empty stage2 results returns empty slice", func(t *testing.T) {
		got, _ := CalculateAggregateRankings(nil, map[string]string{})
		if len(got) != 0 {
			t.Errorf("expected empty slice, got %v", got)
		}
	})

	t.Run("model with no parsed rankings is absent from output", func(t *testing.T) {
		stage2 := []StageTwoResult{
			{Model: "m1", ParsedRanking: []string{}},
		}
		got, _ := CalculateAggregateRankings(stage2, map[string]string{})
		if len(got) != 0 {
			t.Errorf("expected empty output when no rankings parsed, got %v", got)
		}
	})
}

func TestKendallW(t *testing.T) {
	t.Run("perfect agreement returns 1.0", func(t *testing.T) {
		stage2 := []StageTwoResult{
			{Model: "m1", ParsedRanking: []string{"Response A", "Response B", "Response C"}},
			{Model: "m2", ParsedRanking: []string{"Response A", "Response B", "Response C"}},
			{Model: "m3", ParsedRanking: []string{"Response A", "Response B", "Response C"}},
		}
		labelToModel := map[string]string{
			"Response A": "alpha",
			"Response B": "beta",
			"Response C": "gamma",
		}
		_, w := CalculateAggregateRankings(stage2, labelToModel)
		if w != 1.0 {
			t.Errorf("expected W=1.0 for perfect agreement, got %v", w)
		}
	})

	t.Run("full disagreement (two rankers, two items) returns 0.0", func(t *testing.T) {
		// Ranker 1: A first, B second. Ranker 2: B first, A second.
		stage2 := []StageTwoResult{
			{Model: "m1", ParsedRanking: []string{"Response A", "Response B"}},
			{Model: "m2", ParsedRanking: []string{"Response B", "Response A"}},
		}
		labelToModel := map[string]string{"Response A": "alpha", "Response B": "beta"}
		_, w := CalculateAggregateRankings(stage2, labelToModel)
		if w != 0.0 {
			t.Errorf("expected W=0.0 for full disagreement, got %v", w)
		}
	})

	t.Run("single ranker returns 0.0", func(t *testing.T) {
		stage2 := []StageTwoResult{
			{Model: "m1", ParsedRanking: []string{"Response A", "Response B"}},
		}
		labelToModel := map[string]string{"Response A": "alpha", "Response B": "beta"}
		_, w := CalculateAggregateRankings(stage2, labelToModel)
		if w != 0.0 {
			t.Errorf("expected W=0.0 for single ranker, got %v", w)
		}
	})

	t.Run("single item returns 0.0", func(t *testing.T) {
		stage2 := []StageTwoResult{
			{Model: "m1", ParsedRanking: []string{"Response A"}},
			{Model: "m2", ParsedRanking: []string{"Response A"}},
		}
		labelToModel := map[string]string{"Response A": "alpha"}
		_, w := CalculateAggregateRankings(stage2, labelToModel)
		if w != 0.0 {
			t.Errorf("expected W=0.0 for single item, got %v", w)
		}
	})

	t.Run("no rankers returns 0.0", func(t *testing.T) {
		_, w := CalculateAggregateRankings(nil, map[string]string{"Response A": "alpha", "Response B": "beta"})
		if w != 0.0 {
			t.Errorf("expected W=0.0 for no rankers, got %v", w)
		}
	})

	t.Run("incomplete rankings stay within [0,1]", func(t *testing.T) {
		// Both rankers omit the same item — midrank assignment must keep W in [0,1].
		stage2 := []StageTwoResult{
			{Model: "m1", ParsedRanking: []string{"Response A", "Response B"}}, // omits C
			{Model: "m2", ParsedRanking: []string{"Response A", "Response B"}}, // omits C
		}
		labelToModel := map[string]string{
			"Response A": "alpha",
			"Response B": "beta",
			"Response C": "gamma",
		}
		_, w := CalculateAggregateRankings(stage2, labelToModel)
		if w < 0 || w > 1 {
			t.Errorf("W out of [0,1] range for incomplete rankings: %v", w)
		}
		// Both rankers agree on A>B and both omit C — W should be 1.0.
		if w != 1.0 {
			t.Errorf("expected W=1.0 when rankers agree on partial rankings, got %v", w)
		}
	})
}
