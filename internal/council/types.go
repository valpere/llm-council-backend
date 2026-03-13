package council

type StageOneResult struct {
	Model    string `json:"model"`
	Response string `json:"response"`
}

type StageTwoResult struct {
	Model         string   `json:"model"`
	Ranking       string   `json:"ranking"`
	ParsedRanking []string `json:"parsed_ranking"`
}

type StageThreeResult struct {
	Model    string `json:"model"`
	Response string `json:"response"`
}

type AggregateRanking struct {
	Model         string  `json:"model"`
	AverageRank   float64 `json:"average_rank"`
	RankingsCount int     `json:"rankings_count"`
}

type Metadata struct {
	LabelToModel      map[string]string  `json:"label_to_model"`
	AggregateRankings []AggregateRanking `json:"aggregate_rankings"`
}

type Result struct {
	Stage1   []StageOneResult `json:"stage1"`
	Stage2   []StageTwoResult `json:"stage2"`
	Stage3   StageThreeResult `json:"stage3"`
	Metadata Metadata         `json:"metadata"`
}
