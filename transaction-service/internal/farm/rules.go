// Package farm holds the pure farm rules: pest planning, harvest yield and the
// crop pending claim decision. Everything that touches the database lives in
// package econ; these functions are deterministic given their inputs.
package farm

import "time"

// Rand is the randomness source, injected so tests are deterministic.
type Rand interface {
	Float64() float64     // [0, 1)
	Int64N(n int64) int64 // [0, n)
}

// PestPlan is decided once, when a crop is planted, and persisted (P0-097,
// docs/05 §7): reconnects, restarts or restores never roll again.
type PestPlan struct {
	SpawnAt        time.Time
	DeadlineAt     time.Time
	PenaltyPercent int
}

// RollPest decides whether this planting gets its (single) pest.
//
//   - probability is the per-crop Admin value (P0-099);
//   - the spawn time is uniform within the first 70% of the growth time
//     (P0-098);
//   - the catch deadline is 30% of the FULL growth time after the spawn
//     (P0-097), so it is never later than matures_at (F-12);
//   - the penalty is an integer percentage from {5,...,10} (P0-100).
//
// Times are whole microseconds, PostgreSQL's timestamp precision.
func RollPest(r Rand, plantedAt time.Time, growthSeconds int, probability float64) *PestPlan {
	if probability <= 0 || r.Float64() >= probability {
		return nil
	}
	growthMicros := int64(growthSeconds) * 1_000_000
	window := growthMicros * 7 / 10
	spawn := plantedAt.Add(time.Duration(r.Int64N(window+1)) * time.Microsecond)
	deadline := spawn.Add(time.Duration(growthMicros*3/10) * time.Microsecond)
	return &PestPlan{
		SpawnAt:        spawn,
		DeadlineAt:     deadline,
		PenaltyPercent: 5 + int(r.Int64N(6)),
	}
}

// PestPenalty is the number of crops lost to an uncaught pest:
// floor(base * percent / 100) (P0-101); rounding down to 0 is fine (P0-102).
func PestPenalty(baseYield, percent int) int {
	return baseYield * percent / 100
}

// FinalYield applies the penalty when the planting had a pest that was not
// caught. By harvest time the deadline has always passed (deadline <= matures_at).
func FinalYield(baseYield int, hadPest, caught bool, penaltyPercent int) int {
	if !hadPest || caught {
		return baseYield
	}
	return baseYield - PestPenalty(baseYield, penaltyPercent)
}

// ClaimOutcome is the decision for one merged pending crop line.
type ClaimOutcome int

const (
	ClaimWhole   ClaimOutcome = iota // the whole line fits: take it all
	ClaimPartial                     // P0-083: line exceeds the biggest bag possible
	ClaimRefused                     // keep the line untouched
)

// DecideClaim implements P0-078 and P0-083/084.
//
// A line is claimed whole or not at all, except when it is larger than the
// biggest capacity the bag could ever have for this crop (maxPossible); then
// as much as fits now is taken. maxPossible <= 0 means that limit is not
// configured (GAP-07) and the exception stays off.
func DecideClaim(lineQty, freeNow, maxPossible int64) (ClaimOutcome, int64) {
	if freeNow >= lineQty {
		return ClaimWhole, lineQty
	}
	if maxPossible > 0 && lineQty > maxPossible && freeNow > 0 {
		return ClaimPartial, freeNow
	}
	return ClaimRefused, 0
}
