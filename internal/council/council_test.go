package council

import (
	"errors"
	"strings"
	"testing"
)

// ── checkQuorum ──────────────────────────────────────────────────────────────

func makeResults(total, successes int) []StageOneResult {
	results := make([]StageOneResult, total)
	for i := range results {
		if i >= successes {
			results[i].Error = errors.New("failed")
		}
	}
	return results
}

func TestCheckQuorum_N4_ThreeSucceed(t *testing.T) {
	// N=4: need = max(2, ⌈4/2⌉+1) = max(2,3) = 3. Three successes → passes.
	got, err := checkQuorum(makeResults(4, 3), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("len: got %d, want 3", len(got))
	}
}

func TestCheckQuorum_N4_TwoSucceed(t *testing.T) {
	// N=4, need=3. Two successes → QuorumError.
	_, err := checkQuorum(makeResults(4, 2), 0)
	var qe *QuorumError
	if !errors.As(err, &qe) {
		t.Fatalf("expected *QuorumError, got %T: %v", err, err)
	}
	if qe.Got != 2 || qe.Need != 3 {
		t.Errorf("QuorumError{Got:%d Need:%d}, want {2 3}", qe.Got, qe.Need)
	}
}

func TestCheckQuorum_N4_OneSucceeds(t *testing.T) {
	_, err := checkQuorum(makeResults(4, 1), 0)
	var qe *QuorumError
	if !errors.As(err, &qe) {
		t.Fatalf("expected *QuorumError, got %T: %v", err, err)
	}
	if qe.Got != 1 {
		t.Errorf("Got: %d, want 1", qe.Got)
	}
}

func TestCheckQuorum_N2_TwoSucceed(t *testing.T) {
	// N=2: need = max(2, ⌈2/2⌉+1) = max(2,2) = 2. Two successes → passes.
	got, err := checkQuorum(makeResults(2, 2), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len: got %d, want 2", len(got))
	}
}

func TestCheckQuorum_N2_OneSucceeds(t *testing.T) {
	// N=2, need=2. One success → QuorumError.
	_, err := checkQuorum(makeResults(2, 1), 0)
	var qe *QuorumError
	if !errors.As(err, &qe) {
		t.Fatalf("expected *QuorumError, got %T: %v", err, err)
	}
}

func TestCheckQuorum_MinQuorumOverride(t *testing.T) {
	// minQuorum=2 overrides formula; N=4 normally needs 3.
	got, err := checkQuorum(makeResults(4, 2), 2)
	if err != nil {
		t.Fatalf("unexpected error with override: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len: got %d, want 2", len(got))
	}
}

// ── assignLabels ─────────────────────────────────────────────────────────────

func TestAssignLabels_ConsistentMaps(t *testing.T) {
	models := []string{"openai/gpt-4o", "anthropic/claude-haiku", "google/gemini-flash"}
	ltm, mtl := assignLabels(models)

	if len(ltm) != 3 {
		t.Fatalf("labelToModel len: got %d, want 3", len(ltm))
	}
	if len(mtl) != 3 {
		t.Fatalf("modelToLabel len: got %d, want 3", len(mtl))
	}

	// Every label must map to a model and back.
	for label, model := range ltm {
		if mtl[model] != label {
			t.Errorf("round-trip failed: ltm[%q]=%q but mtl[%q]=%q",
				label, model, model, mtl[model])
		}
	}

	// Every model must appear exactly once.
	seen := make(map[string]bool)
	for _, model := range ltm {
		if seen[model] {
			t.Errorf("model %q appears more than once", model)
		}
		seen[model] = true
	}
	for _, m := range models {
		if !seen[m] {
			t.Errorf("model %q missing from labelToModel", m)
		}
	}
}

func TestAssignLabels_LabelsAreLetters(t *testing.T) {
	models := []string{"m1", "m2", "m3"}
	ltm, _ := assignLabels(models)

	wantLabels := map[string]bool{"Response A": true, "Response B": true, "Response C": true}
	for label := range ltm {
		if !wantLabels[label] {
			t.Errorf("unexpected label %q", label)
		}
	}
}

// ── BuildStage3Prompt (W-guidance injection) ─────────────────────────────────

func TestBuildStage3Prompt_StrongConsensus(t *testing.T) {
	msgs := BuildStage3Prompt("Why is the sky blue?", nil, nil, 0.75, nil)
	if len(msgs) == 0 {
		t.Fatal("expected non-empty messages")
	}
	content := msgs[0].Content
	if !strings.Contains(content, "strong consensus") {
		t.Errorf("W=0.75: expected 'strong consensus' in prompt, got:\n%s", content)
	}
}

func TestBuildStage3Prompt_ModerateConsensus(t *testing.T) {
	msgs := BuildStage3Prompt("Why is the sky blue?", nil, nil, 0.55, nil)
	content := msgs[0].Content
	if !strings.Contains(content, "moderate consensus") {
		t.Errorf("W=0.55: expected 'moderate consensus' in prompt, got:\n%s", content)
	}
}

func TestBuildStage3Prompt_NoConsensus(t *testing.T) {
	msgs := BuildStage3Prompt("Why is the sky blue?", nil, nil, 0.30, nil)
	content := msgs[0].Content
	if !strings.Contains(content, "did not reach consensus") {
		t.Errorf("W=0.30: expected 'did not reach consensus' in prompt, got:\n%s", content)
	}
	if !strings.Contains(content, "minority") {
		t.Errorf("W=0.30: expected minority-view guidance in prompt, got:\n%s", content)
	}
}

func TestBuildStage3Prompt_StructuredAttribution(t *testing.T) {
	rankings := []StageTwoResult{
		{ReviewerLabel: "Response A", Rankings: []string{"Response B", "Response C"}},
	}
	labelToModel := map[string]string{
		"Response B": "openai/gpt-4o",
		"Response C": "google/gemini-flash",
	}
	msgs := BuildStage3Prompt("test query", rankings, labelToModel, 0.72, nil)
	content := msgs[0].Content

	if !strings.Contains(content, "openai/gpt-4o") {
		t.Errorf("expected model name in structured attribution, got:\n%s", content)
	}
	// Raw ranking text must NOT contain any LLM-generated prose from Stage 2.
	// Only structured label + model pairs should appear.
	if strings.Contains(content, "StageTwoResult") {
		t.Errorf("raw struct leaked into prompt: %s", content)
	}
}
