package farm

import (
	"math/rand/v2"
	"testing"
	"time"
)

type fixedRand struct {
	f    float64
	ints []int64
}

func (r *fixedRand) Float64() float64 { return r.f }
func (r *fixedRand) Int64N(n int64) int64 {
	v := r.ints[0]
	r.ints = r.ints[1:]
	if v >= n {
		panic("test value out of range")
	}
	return v
}

var planted = time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC)

func TestRollPestRespectsProbability(t *testing.T) {
	if p := RollPest(&fixedRand{f: 0.5}, planted, 1800, 0.5); p != nil {
		t.Fatal("0.5 >= 0.5 must mean no pest")
	}
	if p := RollPest(&fixedRand{f: 0.0}, planted, 1800, 0); p != nil {
		t.Fatal("probability 0 must never spawn")
	}
	if p := RollPest(&fixedRand{f: 0.49, ints: []int64{0, 0}}, planted, 1800, 0.5); p == nil {
		t.Fatal("0.49 < 0.5 must spawn")
	}
}

func TestF12SpawnWindowAndDeadline(t *testing.T) {
	growth := 1800 // 30 minutes
	gMicros := int64(growth) * 1_000_000
	// Latest possible spawn: exactly 70% of the growth time.
	late := RollPest(&fixedRand{f: 0, ints: []int64{gMicros * 7 / 10, 5}}, planted, growth, 1)
	matures := planted.Add(time.Duration(growth) * time.Second)
	if want := planted.Add(21 * time.Minute); !late.SpawnAt.Equal(want) {
		t.Fatalf("spawn %v want %v", late.SpawnAt, want)
	}
	// Deadline is 30% of the FULL growth time (9 min) after the spawn.
	if want := late.SpawnAt.Add(9 * time.Minute); !late.DeadlineAt.Equal(want) {
		t.Fatalf("deadline %v want %v", late.DeadlineAt, want)
	}
	if late.DeadlineAt.After(matures) {
		t.Fatal("deadline after matures_at")
	}
	if late.PenaltyPercent != 10 {
		t.Fatalf("penalty %d", late.PenaltyPercent)
	}
}

func TestRollPestRandomizedInvariants(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for _, growth := range []int{1800, 3600, 7200, 10800, 14400, 7} {
		matures := planted.Add(time.Duration(growth) * time.Second)
		for i := 0; i < 2000; i++ {
			p := RollPest(r, planted, growth, 1)
			if p == nil {
				t.Fatal("probability 1 must always spawn")
			}
			if p.SpawnAt.Before(planted) || p.SpawnAt.Sub(planted) > time.Duration(growth)*time.Second*7/10 {
				t.Fatalf("spawn outside first 70%%: %v", p.SpawnAt.Sub(planted))
			}
			if p.DeadlineAt.After(matures) || !p.DeadlineAt.After(p.SpawnAt) {
				t.Fatalf("bad deadline %v (matures %v)", p.DeadlineAt, matures)
			}
			if p.PenaltyPercent < 5 || p.PenaltyPercent > 10 {
				t.Fatalf("penalty %d", p.PenaltyPercent)
			}
			if p.SpawnAt.Nanosecond()%1000 != 0 || p.DeadlineAt.Nanosecond()%1000 != 0 {
				t.Fatal("times must be whole microseconds")
			}
		}
	}
}

func TestF13YieldPenalty(t *testing.T) {
	cases := []struct {
		base, pct, want int
	}{
		{100, 5, 95}, {100, 10, 90}, {19, 5, 19}, {10, 9, 10}, {10, 10, 9}, {1, 10, 1}, {37, 7, 35},
	}
	for _, c := range cases {
		if got := FinalYield(c.base, true, false, c.pct); got != c.want {
			t.Errorf("base %d pct %d: got %d want %d", c.base, c.pct, got, c.want)
		}
	}
	if FinalYield(100, true, true, 10) != 100 {
		t.Error("caught pest must not reduce yield")
	}
	if FinalYield(100, false, false, 0) != 100 {
		t.Error("no pest, no penalty")
	}
}

func TestDecideClaim(t *testing.T) {
	cases := []struct {
		name                   string
		line, free, maxPossble int64
		want                   ClaimOutcome
		take                   int64
	}{
		{"fits", 120, 120, 0, ClaimWhole, 120},
		{"F-06 refuse whole line", 120, 100, 29970, ClaimRefused, 0},
		{"F-08 exception partial", 40000, 1500, 29970, ClaimPartial, 1500},
		{"exception but bag full", 40000, 0, 29970, ClaimRefused, 0},
		{"limit unknown keeps exception off", 40000, 1500, 0, ClaimRefused, 0},
		{"equal to max is not over it", 29970, 100, 29970, ClaimRefused, 0},
	}
	for _, c := range cases {
		got, take := DecideClaim(c.line, c.free, c.maxPossble)
		if got != c.want || take != c.take {
			t.Errorf("%s: got %v/%d want %v/%d", c.name, got, take, c.want, c.take)
		}
	}
}
