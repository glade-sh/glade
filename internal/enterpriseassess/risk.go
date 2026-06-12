package enterpriseassess

import "github.com/glade-sh/glade/internal/enterprise"

type RiskInputs struct {
	Symbol           string
	TriggerPath      bool
	FanOut           int
	FanIn            int
	DMLOperations    int
	SOQLStatements   int
	PublicOrGlobal   bool
	HasTestIndicator bool
	DynamicReference bool
}

type RiskScore struct {
	Symbol       string
	Score        int
	Severity     enterprise.Severity
	Explanations []string
}

func ScoreNode(in RiskInputs) RiskScore {
	score := RiskScore{Symbol: in.Symbol}
	if in.TriggerPath {
		score.Score += 20
		score.Explanations = append(score.Explanations, "+20 trigger path")
	}
	if in.FanOut >= 10 {
		score.Score += 15
		score.Explanations = append(score.Explanations, "+15 fan-out >= 10")
	}
	if in.FanIn >= 10 {
		score.Score += 10
		score.Explanations = append(score.Explanations, "+10 fan-in >= 10")
	}
	if in.DMLOperations > 0 {
		score.Score += 15
		score.Explanations = append(score.Explanations, "+15 DML > 0")
	}
	if in.SOQLStatements >= 3 {
		score.Score += 10
		score.Explanations = append(score.Explanations, "+10 SOQL >= 3")
	}
	if in.PublicOrGlobal {
		score.Score += 10
		score.Explanations = append(score.Explanations, "+10 public/global visibility")
	}
	if !in.HasTestIndicator {
		score.Score += 10
		score.Explanations = append(score.Explanations, "+10 no test indicator")
	}
	if in.DynamicReference {
		score.Score += 10
		score.Explanations = append(score.Explanations, "+10 dynamic reference")
	}
	score.Severity = SeverityForScore(score.Score)
	return score
}

func SeverityForScore(score int) enterprise.Severity {
	switch {
	case score >= 60:
		return enterprise.SeverityHigh
	case score >= 35:
		return enterprise.SeverityMedium
	case score >= 15:
		return enterprise.SeverityLow
	default:
		return enterprise.SeverityInfo
	}
}
