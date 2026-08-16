package tweak

import "winforge/internal/config"

// Health is the dashboard "system health" score and its contributing factors.
type Health struct {
	Score              int `json:"score"`
	TotalTweaks        int `json:"totalTweaks"`
	AppliedTweaks      int `json:"appliedTweaks"`
	UnappliedLow       int `json:"unappliedLow"`
	UnappliedMedium    int `json:"unappliedMedium"`
	UnappliedHigh      int `json:"unappliedHigh"`
	UnverifiableTweaks int `json:"unverifiableTweaks,omitempty"`
	BloatwareCount     int `json:"bloatwareCount"`
}

// ComputeHealth applies the ENGINE health algorithm:
//
//	100 - (unapplied_low * 2) - (unapplied_medium * 5) - (unapplied_high * 10) - (bloatware * 3)
//
// The score is clamped to [0, 100].
//
// This is intentionally different from the Next.js simulation's formula in
// src/lib/health.ts (baseline 50 with capped bonuses and penalties). The two
// surfaces score different inputs — the engine reads real registry/service
// state, the web app reads a simulated PostgreSQL catalog — so they are not
// expected to produce identical numbers. Each formula has tests pinning its
// behavior. Do not "align" them without updating both implementations and
// their tests together.
func ComputeHealth(tweaks []config.Tweak, applied map[string]bool, bloatware int) Health {
	h := Health{
		Score:          100,
		TotalTweaks:    len(tweaks),
		BloatwareCount: bloatware,
	}

	for _, t := range tweaks {
		if applied[t.ID] {
			h.AppliedTweaks++
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
