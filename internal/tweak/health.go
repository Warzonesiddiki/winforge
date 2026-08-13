package tweak

import "winforge/internal/config"

// Health is the dashboard "system health" score and its contributing factors.
type Health struct {
	Score           int `json:"score"`
	TotalTweaks     int `json:"totalTweaks"`
	AppliedTweaks   int `json:"appliedTweaks"`
	UnappliedLow    int `json:"unappliedLow"`
	UnappliedMedium int `json:"unappliedMedium"`
	UnappliedHigh   int `json:"unappliedHigh"`
	BloatwareCount  int `json:"bloatwareCount"`
}

// ComputeHealth applies the health algorithm:
//
//	100 - (unapplied_low * 2) - (unapplied_medium * 5) - (bloatware * 3)
//
// Unapplied high-risk tweaks additionally subtract 10 each. The score is
// clamped to [0, 100].
func ComputeHealth(tweaks []config.Tweak, applied map[string]bool, bloatware int) Health {
	h := Health{
		Score:          100,
		TotalTweaks:    len(tweaks),
		AppliedTweaks:  len(applied),
		BloatwareCount: bloatware,
	}

	for _, t := range tweaks {
		if applied[t.ID] {
			continue
		}
		switch t.Risk {
		case config.RiskLow:
			h.UnappliedLow++
		case config.RiskMedium:
			h.UnappliedMedium++
		case config.RiskHigh:
			h.UnappliedHigh++
		}
	}

	h.Score = 100 -
		h.UnappliedLow*2 -
		h.UnappliedMedium*5 -
		h.UnappliedHigh*10 -
		bloatware*3
	if h.Score < 0 {
		h.Score = 0
	}
	if h.Score > 100 {
		h.Score = 100
	}
	return h
}
