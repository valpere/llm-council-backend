package council

import (
	"fmt"
	"sort"
	"strings"
)

// BuildStage0GeneratorPrompt returns the prompt for Stage 0 generator queries.
// Generators must return JSON: {"questions": [{"text": "..."}]}
func BuildStage0GeneratorPrompt(query string, history []ClarificationRound) []ChatMessage {
	var sb strings.Builder
	sb.WriteString("You are helping clarify a question before a council of AI models answers it.\n")
	sb.WriteString("Original question: ")
	sb.WriteString(query)
	sb.WriteString("\n")

	if len(history) > 0 {
		sb.WriteString("\nPrior clarification Q&A:\n")
		for _, r := range history {
			for i, q := range r.Questions {
				sb.WriteString("Q: ")
				sb.WriteString(q.Text)
				sb.WriteString("\n")
				answer := "(no answer)"
				for _, a := range r.Answers {
					if a.ID == q.ID && a.Text != "" {
						answer = a.Text
						break
					}
				}
				// Also check positional match if IDs don't line up
				if answer == "(no answer)" && i < len(r.Answers) && r.Answers[i].Text != "" {
					answer = r.Answers[i].Text
				}
				sb.WriteString("A: ")
				sb.WriteString(answer)
				sb.WriteString("\n\n")
			}
		}
	}

	sb.WriteString("\nIdentify contradictions, ambiguities, or missing context in the question.\n")
	sb.WriteString("Return ONLY a JSON object: {\"questions\": [{\"text\": \"...\"}]}\n")
	sb.WriteString("Return an empty questions array if the question is already clear enough.")

	return []ChatMessage{
		{Role: "user", Content: sb.String()},
	}
}

// BuildStage0ChairmanPrompt returns the prompt for the Stage 0 chairman decision.
// Chairman must return JSON: {"questions": [{"id": "q1", "text": "..."}], "enough": true/false}
func BuildStage0ChairmanPrompt(query string, candidates []string, round, maxRounds, maxPerRound, accumulated, maxTotal int) []ChatMessage {
	var sb strings.Builder
	sb.WriteString("You are deciding whether to ask the user for clarification before answering.\n")
	sb.WriteString("Original question: ")
	sb.WriteString(query)
	sb.WriteString("\n\n")

	if len(candidates) > 0 {
		sb.WriteString("Proposed clarification questions:\n")
		for _, c := range candidates {
			sb.WriteString("- ")
			sb.WriteString(c)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("No clarification questions were proposed.\n\n")
	}

	fmt.Fprintf(&sb, "Current round: %d/%d, Questions asked so far: %d/%d\n", round, maxRounds, accumulated, maxTotal)
	fmt.Fprintf(&sb, "Select up to %d most important questions, merge duplicates.\n", maxPerRound)
	sb.WriteString("If the question is clear enough or more clarification would not significantly improve the answer, set 'enough': true.\n")
	sb.WriteString("Return ONLY JSON: {\"questions\": [{\"id\": \"q1\", \"text\": \"...\"}, ...], \"enough\": false}\n")
	fmt.Fprintf(&sb, "Use sequential IDs starting from q%d.", accumulated+1)

	return []ChatMessage{
		{Role: "user", Content: sb.String()},
	}
}

// BuildAugmentedQuery builds the full query passed to RunFull when clarification history exists.
func BuildAugmentedQuery(query string, history []ClarificationRound) string {
	if len(history) == 0 {
		return query
	}

	// Check if any round has at least one non-empty answer
	hasAnswers := false
	for _, r := range history {
		for _, a := range r.Answers {
			if a.Text != "" {
				hasAnswers = true
				break
			}
		}
		if hasAnswers {
			break
		}
	}
	if !hasAnswers {
		return query
	}

	var sb strings.Builder
	sb.WriteString(query)
	sb.WriteString("\n\n## User clarifications\n")

	for _, r := range history {
		if len(r.Answers) == 0 {
			continue
		}
		// Check if this round has any non-empty answer
		roundHasAnswers := false
		for _, a := range r.Answers {
			if a.Text != "" {
				roundHasAnswers = true
				break
			}
		}
		if !roundHasAnswers {
			continue
		}
		for _, q := range r.Questions {
			answer := "(no answer)"
			for _, a := range r.Answers {
				if a.ID == q.ID {
					if a.Text != "" {
						answer = a.Text
					}
					break
				}
			}
			sb.WriteString("Q: ")
			sb.WriteString(q.Text)
			sb.WriteString("\nA: ")
			sb.WriteString(answer)
			sb.WriteString("\n\n")
		}
	}

	return sb.String()
}

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
			fmt.Fprintf(&sb, "  %d. %s (%s)\n", i+1, label, model)
		}
	}

	sb.WriteString("\nProvide a comprehensive, well-reasoned answer that synthesizes the best insights from all responses.")

	return []ChatMessage{
		{Role: "user", Content: sb.String()},
	}
}
