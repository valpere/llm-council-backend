package council

import (
	"fmt"
	"sort"
	"strings"
)

// BuildStage1Prompt returns the messages for a Stage 1 generation request.
func BuildStage1Prompt(query string) []ChatMessage {
	return []ChatMessage{
		{Role: "user", Content: query},
	}
}

// BuildStage2Prompt returns the messages for a Stage 2 peer-review request.
// labeledResponses maps anonymous label → response text (e.g. "Response A" → "...").
// The prompt requests JSON output with schema {"rankings": ["Response X", ...]}.
func BuildStage2Prompt(query string, labeledResponses map[string]string) []ChatMessage {
	// Sort labels for a deterministic, readable prompt.
	labels := make([]string, 0, len(labeledResponses))
	for l := range labeledResponses {
		labels = append(labels, l)
	}
	sort.Strings(labels)

	var sb strings.Builder
	sb.WriteString("You were asked the following question:\n\n")
	sb.WriteString(query)
	sb.WriteString("\n\nHere are the responses given by different assistants:\n\n")
	for _, label := range labels {
		sb.WriteString("## ")
		sb.WriteString(label)
		sb.WriteString("\n")
		sb.WriteString(labeledResponses[label])
		sb.WriteString("\n\n")
	}
	sb.WriteString("Rank these responses from best to worst based on accuracy, clarity, and completeness.\n")
	sb.WriteString("Return ONLY a JSON object with this exact schema — no additional text:\n")
	sb.WriteString(`{"rankings": ["Response X", "Response Y", ...]}`)
	sb.WriteString("\n\nList all response labels in order from best (first) to worst (last).")

	return []ChatMessage{
		{Role: "user", Content: sb.String()},
	}
}

// BuildStage3Prompt returns the messages for the Stage 3 Chairman synthesis request.
// labeledResponses contains the Stage 1 candidate answers (label → content).
// Rankings are built from Go structs — Stage 2 reviewer prose is never passed through,
// preventing prompt injection from Stage 2 model output.
// Kendall's W drives the synthesis guidance injected into the prompt.
func BuildStage3Prompt(query string, rankings []StageTwoResult, labelToModel map[string]string, consensusW float64, labeledResponses map[string]string) []ChatMessage {
	var guidance string
	switch {
	case consensusW >= 0.70:
		guidance = fmt.Sprintf(
			"The peer reviewers reached strong consensus (W=%.2f). "+
				"Synthesize the responses confidently, drawing on the most highly-ranked insights.",
			consensusW,
		)
	case consensusW >= 0.40:
		guidance = fmt.Sprintf(
			"The peer reviewers reached moderate consensus (W=%.2f). "+
				"Synthesize the best insights while acknowledging where reviewers diverged.",
			consensusW,
		)
	default:
		guidance = fmt.Sprintf(
			"The peer reviewers did not reach consensus (W=%.2f). "+
				"Present the main perspectives fairly, surface well-reasoned minority views, "+
				"and help the user understand the range of expert opinion.",
			consensusW,
		)
	}

	var sb strings.Builder
	sb.WriteString("You were asked to answer:\n\n")
	sb.WriteString(query)

	// Include Stage 1 candidate responses so the Chairman can synthesize their content.
	if len(labeledResponses) > 0 {
		labels := make([]string, 0, len(labeledResponses))
		for l := range labeledResponses {
			labels = append(labels, l)
		}
		sort.Strings(labels)
		sb.WriteString("\n\nCandidate responses:\n")
		for _, label := range labels {
			sb.WriteString("\n## ")
			sb.WriteString(label)
			sb.WriteString("\n")
			sb.WriteString(labeledResponses[label])
		}
	}

	sb.WriteString("\n\n")
	sb.WriteString(guidance)
	sb.WriteString("\n\nPeer review rankings (structured attribution — best to worst):\n")

	for _, r := range rankings {
		if len(r.Rankings) == 0 {
			continue
		}
		sb.WriteString("\nReviewer ")
		sb.WriteString(r.ReviewerLabel)
		sb.WriteString(":\n")
		for i, label := range r.Rankings {
			model := labelToModel[label]
			sb.WriteString(fmt.Sprintf("  %d. %s (%s)\n", i+1, label, model))
		}
	}

	sb.WriteString("\nProvide a comprehensive, well-reasoned answer that synthesizes the best insights from all responses.")

	return []ChatMessage{
		{Role: "user", Content: sb.String()},
	}
}
