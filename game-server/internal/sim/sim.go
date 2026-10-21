// Package sim is the deterministic movement step shared, rule for rule, with
// the client's prediction (client/src/game/sim/movement.ts). Everything is
// integer math in 1/100 pixel so Go and JavaScript produce identical results;
// shared/fixtures/movement_cases.json is checked by both test suites.
//
// One input = one step: the direction (scaled by 1000, clamped to length 1000)
// times SpeedPerStep, moved along x then along y in sub-steps of at most one
// pixel; an axis stops at the first sub-step that would collide.
package sim

import "math"

const (
	// Unit is the number of position units per pixel.
	Unit = 100
	// DirScale is the length of a full-speed direction vector.
	DirScale = 1000
	// SubStep bounds each collision probe to one pixel.
	SubStep = 1 * Unit
	// The player's collider: a box centred on the feet.
	HalfWidth  = 9 * Unit
	HalfHeight = 5 * Unit
	// maxComponent rejects garbage input before any multiplication.
	maxComponent = 1 << 20
)

// Rect is an axis-aligned blocked area, half-open: [X0, X1) x [Y0, Y1).
type Rect struct{ X0, Y0, X1, Y1 int32 }

// World is the static collision data of one map.
type World struct {
	Width, Height int32 // map size in units
	TileSize      int32 // in units
	Cols, Rows    int32
	Blocked       []bool // Cols*Rows, row-major
	Solids        []Rect // NPC colliders
}

// ClampDir returns the direction actually used for a step. Vectors up to
// 1001 long pass unchanged (so the rule is idempotent); longer ones are
// scaled to ~1000.
func ClampDir(dx, dy int32) (int32, int32) {
	if dx > maxComponent || dx < -maxComponent || dy > maxComponent || dy < -maxComponent {
		return 0, 0
	}
	l2 := int64(dx)*int64(dx) + int64(dy)*int64(dy)
	if l2 <= 1001*1001 {
		return dx, dy
	}
	l := int64(math.Floor(math.Sqrt(float64(l2))))
	return int32(int64(dx) * DirScale / l), int32(int64(dy) * DirScale / l)
}

// Collides reports whether the player box with feet at (x, y) overlaps a
// blocked tile, a solid, or leaves the map.
func (w *World) Collides(x, y int32) bool {
	x0, x1 := x-HalfWidth, x+HalfWidth
	y0, y1 := y-HalfHeight, y+HalfHeight
	if x0 < 0 || y0 < 0 || x1 > w.Width || y1 > w.Height {
		return true
	}
	for ty := y0 / w.TileSize; ty <= (y1-1)/w.TileSize; ty++ {
		for tx := x0 / w.TileSize; tx <= (x1-1)/w.TileSize; tx++ {
			if w.Blocked[ty*w.Cols+tx] {
				return true
			}
		}
	}
	for _, s := range w.Solids {
		if x0 < s.X1 && s.X0 < x1 && y0 < s.Y1 && s.Y0 < y1 {
			return true
		}
	}
	return false
}

// Step applies one input. speed is the distance of a full-length step.
func (w *World) Step(x, y, dx, dy, speed int32) (int32, int32) {
	dx, dy = ClampDir(dx, dy)
	mx := int32(int64(dx) * int64(speed) / DirScale)
	my := int32(int64(dy) * int64(speed) / DirScale)
	for mx != 0 {
		s := clamp(mx, SubStep)
		if w.Collides(x+s, y) {
			break
		}
		x += s
		mx -= s
	}
	for my != 0 {
		s := clamp(my, SubStep)
		if w.Collides(x, y+s) {
			break
		}
		y += s
		my -= s
	}
	return x, y
}

func clamp(v, limit int32) int32 {
	if v > limit {
		return limit
	}
	if v < -limit {
		return -limit
	}
	return v
}

// Dir8 maps a direction to 0=S 1=SW 2=W 3=NW 4=N 5=NE 6=E 7=SE; ok is false
// for the zero vector.
func Dir8(dx, dy int32) (uint8, bool) {
	if dx == 0 && dy == 0 {
		return 0, false
	}
	// Angle measured from south (+y), clockwise towards west.
	a := math.Atan2(float64(-dx), float64(dy))
	if a < 0 {
		a += 2 * math.Pi
	}
	return uint8(int(math.Round(a/(math.Pi/4))) % 8), true
}
