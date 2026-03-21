package council

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"time"

	"llm-council/internal/openrouter"
)

type Council struct {
	client        *openrouter.Client
	councilModels []string
	chairmanModel string
}

func New(client *openrouter.Client, councilModels []string, chairmanModel string) *Council {
	return &Council{
		client:        client,
		councilModels: councilModels,
		chairmanModel: chairmanModel,
	}
}

func (c *Council) Stage1CollectResponses(ctx context.Context, userQuery string) ([]StageOneResult, error) {
	messages := []openrouter.Message{{Role: "user", Content: userQuery}}
	modelResults := c.client.QueryModelsParallel(ctx, c.councilModels, messages, 120*time.Second)

	var results []StageOneResult
	for _, mr := range modelResults {
		if mr.Err != nil {
			log.Printf("stage1: error querying %s: %v", mr.Model, mr.Err)
			continue
		}
		results = append(results, StageOneResult{Model: mr.Model, Response: mr.Response.Content})
	}
	return results, nil
}

func (c *Council) Stage2CollectRankings(ctx context.Context, userQuery string, stage1Results []StageOneResult) ([]StageTwoResult, map[string]string, error) {
	if len(stage1Results) > 26 {
		return nil, nil, fmt.Errorf("too many responses for Stage 2: maximum 26 supported, got %d", len(stage1Results))
	}

	// Shuffle the order in which responses are labeled to prevent label-position bias.
	order := rand.Perm(len(stage1Results))

	labelToModel := make(map[string]string, len(stage1Results))
	var responsesText strings.Builder
	for labelIdx, resultIdx := range order {
		label := string(rune('A' + labelIdx))
		result := stage1Results[resultIdx]
		labelToModel["Response "+label] = result.Model
		if labelIdx > 0 {
			responsesText.WriteString("\n\n")
		}
		fmt.Fprintf(&responsesText, "Response %s:\n%s", label, result.Response)
	}

	rankingPrompt := fmt.Sprintf(`You are evaluating different responses to the following question:

Question: %s

Here are the responses from different models (anonymized):

%s

Your task:
1. First, evaluate each response individually. For each response, explain what it does well and what it does poorly.
2. Then, at the very end of your response, provide a final ranking.

IMPORTANT: Your final ranking MUST be formatted EXACTLY as follows:
- Start with the line "FINAL RANKING:" (all caps, with colon)
- Then list the responses from best to worst as a numbered list
- Each line should be: number, period, space, then ONLY the response label (e.g., "1. Response A")
- Do not add any other text or explanations in the ranking section

Example of the correct format for your ENTIRE response:

Response A provides good detail on X but misses Y...
Response B is accurate but lacks depth on Z...
Response C offers the most comprehensive answer...

FINAL RANKING:
1. Response C
2. Response A
3. Response B

Now provide your evaluation and ranking:`, userQuery, responsesText.String())

	messages := []openrouter.Message{{Role: "user", Content: rankingPrompt}}
	modelResults := c.client.QueryModelsParallel(ctx, c.councilModels, messages, 120*time.Second)

	var results []StageTwoResult
	for _, mr := range modelResults {
		if mr.Err != nil {
			log.Printf("stage2: error querying %s: %v", mr.Model, mr.Err)
			continue
		}
		fullText := mr.Response.Content
		results = append(results, StageTwoResult{
			Model:         mr.Model,
			Ranking:       fullText,
			ParsedRanking: parseRankingFromText(fullText),
		})
	}

	return results, labelToModel, nil
}

func (c *Council) Stage3SynthesizeFinal(ctx context.Context, userQuery string, stage1Results []StageOneResult, stage2Results []StageTwoResult) (StageThreeResult, error) {
	var stage1Text strings.Builder
	for i, r := range stage1Results {
		if i > 0 {
			stage1Text.WriteString("\n\n")
		}
		fmt.Fprintf(&stage1Text, "Model: %s\nResponse: %s", r.Model, r.Response)
	}

	var stage2Text strings.Builder
	for i, r := range stage2Results {
		if i > 0 {
			stage2Text.WriteString("\n\n")
		}
		fmt.Fprintf(&stage2Text, "Model: %s\nRanking: %s", r.Model, r.Ranking)
	}

	chairmanPrompt := fmt.Sprintf(`You are the Chairman of an LLM Council. Multiple AI models have provided responses to a user's question, and then ranked each other's responses.

Original Question: %s

STAGE 1 - Individual Responses:
%s

STAGE 2 - Peer Rankings:
%s

Your task as Chairman is to synthesize all of this information into a single, comprehensive, accurate answer to the user's original question. Consider:
- The individual responses and their insights
- The peer rankings and what they reveal about response quality
- Any patterns of agreement or disagreement

Provide a clear, well-reasoned final answer that represents the council's collective wisdom:`,
		userQuery, stage1Text.String(), stage2Text.String())

	messages := []openrouter.Message{{Role: "user", Content: chairmanPrompt}}
	resp, err := c.client.QueryModel(ctx, c.chairmanModel, messages, 120*time.Second)
	if err != nil {
		return StageThreeResult{}, fmt.Errorf("stage3: chairman %s: %w", c.chairmanModel, err)
	}
	return StageThreeResult{Model: c.chairmanModel, Response: resp.Content}, nil
}

func (c *Council) GenerateTitle(ctx context.Context, userQuery string) string {
	prompt := fmt.Sprintf(`Generate a very short title (3-5 words maximum) that summarizes the following question.
The title should be concise and descriptive. Do not use quotes or punctuation in the title.

Question: %s

Title:`, userQuery)

	messages := []openrouter.Message{{Role: "user", Content: prompt}}
	resp, err := c.client.QueryModel(ctx, "google/gemini-2.5-flash", messages, 30*time.Second)
	if err != nil {
		return "New Conversation"
	}
	title := strings.TrimSpace(resp.Content)
	title = strings.Trim(title, `"'`)
	if len(title) > 50 {
		title = title[:47] + "..."
	}
	return title
}

func (c *Council) RunFull(ctx context.Context, userQuery string) (Result, error) {
	stage1, err := c.Stage1CollectResponses(ctx, userQuery)
	if err != nil {
		return Result{}, err
	}
	if len(stage1) == 0 {
		return Result{
			Stage3: StageThreeResult{Model: "error", Response: "All models failed to respond. Please try again."},
		}, nil
	}

	stage2, labelToModel, err := c.Stage2CollectRankings(ctx, userQuery, stage1)
	if err != nil {
		return Result{}, err
	}

	aggregateRankings := CalculateAggregateRankings(stage2, labelToModel)
	stage3, err := c.Stage3SynthesizeFinal(ctx, userQuery, stage1, stage2)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Stage1: stage1,
		Stage2: stage2,
		Stage3: stage3,
		Metadata: Metadata{
			LabelToModel:      labelToModel,
			AggregateRankings: aggregateRankings,
		},
	}, nil
}

var (
	reNumbered    = regexp.MustCompile(`\d+\.\s*Response [A-Z]`)
	reResponseLabel = regexp.MustCompile(`Response [A-Z]`)
)

func parseRankingFromText(text string) []string {
	if idx := strings.Index(text, "FINAL RANKING:"); idx >= 0 {
		section := text[idx+len("FINAL RANKING:"):]
		if numbered := reNumbered.FindAllString(section, -1); len(numbered) > 0 {
			result := make([]string, len(numbered))
			for i, m := range numbered {
				result[i] = reResponseLabel.FindString(m)
			}
			return result
		}
		return reResponseLabel.FindAllString(section, -1)
	}
	return reResponseLabel.FindAllString(text, -1)
}

func CalculateAggregateRankings(stage2Results []StageTwoResult, labelToModel map[string]string) []AggregateRanking {
	modelPositions := make(map[string][]int)
	for _, r := range stage2Results {
		for pos, label := range r.ParsedRanking {
			if model, ok := labelToModel[label]; ok {
				modelPositions[model] = append(modelPositions[model], pos+1)
			}
		}
	}

	aggregates := make([]AggregateRanking, 0, len(modelPositions))
	for model, positions := range modelPositions {
		sum := 0
		for _, p := range positions {
			sum += p
		}
		avg := math.Round(float64(sum)/float64(len(positions))*100) / 100
		aggregates = append(aggregates, AggregateRanking{
			Model:         model,
			AverageRank:   avg,
			RankingsCount: len(positions),
		})
	}

	sort.Slice(aggregates, func(i, j int) bool {
		return aggregates[i].AverageRank < aggregates[j].AverageRank
	})
	return aggregates
}
