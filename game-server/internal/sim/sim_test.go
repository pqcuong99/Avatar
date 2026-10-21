package sim_test

import (
	"encoding/json"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"

	"example.com/avatar/game-server/internal/mapdata"
	"example.com/avatar/game-server/internal/sim"
)

func repoFile(t *testing.T, parts ...string) string {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		p := filepath.Join(append([]string{dir}, parts...)...)
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("%v not found", parts)
		}
		dir = parent
	}
}

func loadPlaza(t *testing.T) *mapdata.Map {
	t.Helper()
	reg, err := mapdata.Load(filepath.Dir(repoFile(t, "shared", "maps", "maps.json")))
	if err != nil {
		t.Fatal(err)
	}
	return reg.Maps["plaza"]
}

func TestClampDirIsIdempotentAndBounded(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	for i := 0; i < 100000; i++ {
		dx, dy := int32(r.IntN(6001)-3000), int32(r.IntN(6001)-3000)
		cx, cy := sim.ClampDir(dx, dy)
		if int64(cx)*int64(cx)+int64(cy)*int64(cy) > 1001*1001 {
			t.Fatalf("clamp(%d,%d) = (%d,%d) too long", dx, dy, cx, cy)
		}
		if ax, ay := sim.ClampDir(cx, cy); ax != cx || ay != cy {
			t.Fatalf("not idempotent: (%d,%d) -> (%d,%d) -> (%d,%d)", dx, dy, cx, cy, ax, ay)
		}
	}
	if x, y := sim.ClampDir(1<<30, 0); x != 0 || y != 0 {
		t.Fatal("garbage input must be dropped")
	}
}

func TestStepNeverEntersWallsOrNPCs(t *testing.T) {
	m := loadPlaza(t)
	r := rand.New(rand.NewPCG(5, 6))
	x, y := m.Spawn[0], m.Spawn[1]
	for i := 0; i < 50000; i++ {
		dx, dy := int32(r.IntN(2001)-1000), int32(r.IntN(2001)-1000)
		nx, ny := m.World.Step(x, y, dx, dy, 800)
		if m.World.Collides(nx, ny) {
			t.Fatalf("step %d ended inside a wall at %d,%d", i, nx, ny)
		}
		if d := abs(nx-x) + abs(ny-y); d > 1600 {
			t.Fatalf("step %d moved %d units (speed hack)", i, d)
		}
		x, y = nx, ny
	}
}

func TestStepStopsAtWall(t *testing.T) {
	// A 3x3 tile world with the middle-right tile blocked.
	w := &sim.World{Width: 9600, Height: 9600, TileSize: 3200, Cols: 3, Rows: 3, Blocked: make([]bool, 9)}
	w.Blocked[1*3+2] = true
	x, y := int32(4800), int32(4800)
	for i := 0; i < 20; i++ {
		x, y = w.Step(x, y, 1000, 0, 800)
	}
	if x != 6400-sim.HalfWidth || y != 4800 {
		t.Fatalf("stopped at %d,%d", x, y)
	}
}

func TestDir8(t *testing.T) {
	cases := map[[2]int32]uint8{{0, 1}: 0, {-1, 1}: 1, {-1, 0}: 2, {-1, -1}: 3, {0, -1}: 4, {1, -1}: 5, {1, 0}: 6, {1, 1}: 7}
	for v, want := range cases {
		if got, ok := sim.Dir8(v[0]*1000, v[1]*1000); !ok || got != want {
			t.Errorf("Dir8(%v) = %d, want %d", v, got, want)
		}
	}
}

func abs(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// Golden cases shared with the client's prediction tests. Regenerate with
// UPDATE_GOLDEN=1 go test ./internal/sim/ after an intended rule change.
type golden struct {
	Map        string     `json:"map"`
	MapVersion string     `json:"map_version"`
	Speed      int32      `json:"speed"`
	Clamp      [][4]int32 `json:"clamp"`
	Walks      []walk     `json:"walks"`
}

type walk struct {
	Start     [2]int32   `json:"start"`
	Inputs    [][2]int32 `json:"inputs"`
	Positions [][2]int32 `json:"positions"`
}

func buildGolden(m *mapdata.Map) golden {
	r := rand.New(rand.NewPCG(2026, 1021))
	g := golden{Map: "plaza.json", MapVersion: m.Version, Speed: 800}
	for i := 0; i < 200; i++ {
		dx, dy := int32(r.IntN(4001)-2000), int32(r.IntN(4001)-2000)
		cx, cy := sim.ClampDir(dx, dy)
		g.Clamp = append(g.Clamp, [4]int32{dx, dy, cx, cy})
	}
	farm := m.NPCs["FARM_SHOP"]
	starts := [][2]int32{m.Spawn, {farm.X, farm.Y + 2000}, {9*3200 + 1600, 19*3200 + 1600}, {38 * 3200, 11 * 3200}, {23 * 3200, 12 * 3200}}
	dirs := [][2]int32{{0, 1000}, {0, -1000}, {1000, 0}, {-1000, 0}, {707, 707}, {-707, 707}, {707, -707}, {-707, -707}, {0, 0}}
	for _, s := range starts {
		if m.World.Collides(s[0], s[1]) {
			panic("golden start collides")
		}
		wk := walk{Start: s}
		x, y := s[0], s[1]
		var cur [2]int32
		for i := 0; i < 400; i++ {
			if i%12 == 0 {
				if r.IntN(3) == 0 {
					cur = [2]int32{int32(r.IntN(3001) - 1500), int32(r.IntN(3001) - 1500)}
				} else {
					cur = dirs[r.IntN(len(dirs))]
				}
			}
			x, y = m.World.Step(x, y, cur[0], cur[1], g.Speed)
			wk.Inputs = append(wk.Inputs, cur)
			wk.Positions = append(wk.Positions, [2]int32{x, y})
		}
		g.Walks = append(g.Walks, wk)
	}
	return g
}

func TestGoldenMovementCases(t *testing.T) {
	m := loadPlaza(t)
	got := buildGolden(m)
	raw, _ := json.Marshal(got)
	path := filepath.Join(filepath.Dir(filepath.Dir(repoFile(t, "shared", "maps", "maps.json"))), "fixtures", "movement_cases.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with UPDATE_GOLDEN=1 to create it)", err)
	}
	var w golden
	if err := json.Unmarshal(want, &w); err != nil {
		t.Fatal(err)
	}
	if w.MapVersion != m.Version {
		t.Fatalf("fixture is for %s, map is %s: regenerate it", w.MapVersion, m.Version)
	}
	for i, c := range w.Clamp {
		if x, y := sim.ClampDir(c[0], c[1]); x != c[2] || y != c[3] {
			t.Fatalf("clamp %d: got %d,%d want %d,%d", i, x, y, c[2], c[3])
		}
	}
	for i, wk := range w.Walks {
		x, y := wk.Start[0], wk.Start[1]
		for j, in := range wk.Inputs {
			x, y = m.World.Step(x, y, in[0], in[1], w.Speed)
			if x != wk.Positions[j][0] || y != wk.Positions[j][1] {
				t.Fatalf("walk %d step %d: got %d,%d want %v", i, j, x, y, wk.Positions[j])
			}
		}
	}
}
