package textanalyzer

type AnalysisHintRegistry struct {
	Fields []AnalysisFieldHint
}

type AnalysisFieldHint struct {
	Key           string
	Label         string
	MultiValue    bool
	Priority      int
	ExclusiveWith []string
	Values        []AnalysisValueHint
}

type AnalysisValueHint struct {
	CanonicalValue string
	Aliases        []string
	Weight         int
	Source         string
}

type AnalyzeTextRequest struct {
	Input               string
	AllowFuzzyMatch     bool
	AutoRepairBrackets  bool
	SplitByPathSegments bool
	PreferLongestMatch  bool
	MaxResults          int
}

type AnalyzeTextResult struct {
	Input           string
	NormalizedInput string
	ResidualText    string
	TitleCandidate  string
	Fields          map[string][]string
	Matches         []AnalyzeMatch
	Rejected        []AnalyzeRejectedMatch
	Warnings        []string
}

type AnalyzeMatch struct {
	Key         string
	Value       string
	Alias       string
	Source      string
	Confidence  float64
	Start       int
	End         int
	MatchedText string
}

type AnalyzeRejectedMatch struct {
	Key         string
	Value       string
	Reason      string
	MatchedText string
}

type Analyzer interface {
	Analyze(req AnalyzeTextRequest, registry AnalysisHintRegistry) (AnalyzeTextResult, error)
}
