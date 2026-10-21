// Package mapdata loads Tiled JSON maps (P0-167): the collision layer and the
// NPC colliders become the sim.World the Game Server validates movement with,
// the same file the client renders.
package mapdata

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"example.com/avatar/game-server/internal/sim"
)

type NPC struct {
	Code           string
	X, Y           int32 // feet position, units
	Collider       sim.Rect
	InteractRadius int32 // units
}

type Map struct {
	ID          string
	Version     string
	DisplayName string
	Public      bool
	World       *sim.World
	NPCs        map[string]NPC
	Spawn       [2]int32 // units
}

type Registry struct {
	Default string
	Maps    map[string]*Map
}

type tiledProperty struct {
	Name  string          `json:"name"`
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

type tiledObject struct {
	Name       string          `json:"name"`
	Type       string          `json:"type"`
	X          float64         `json:"x"`
	Y          float64         `json:"y"`
	Width      float64         `json:"width"`
	Height     float64         `json:"height"`
	Point      bool            `json:"point"`
	Properties []tiledProperty `json:"properties"`
}

type tiledLayer struct {
	Name    string        `json:"name"`
	Type    string        `json:"type"`
	Width   int           `json:"width"`
	Height  int           `json:"height"`
	Data    []int         `json:"data"`
	Objects []tiledObject `json:"objects"`
}

type tiledMap struct {
	Width      int             `json:"width"`
	Height     int             `json:"height"`
	TileWidth  int             `json:"tilewidth"`
	TileHeight int             `json:"tileheight"`
	Infinite   bool            `json:"infinite"`
	Properties []tiledProperty `json:"properties"`
	Layers     []tiledLayer    `json:"layers"`
}

func prop(props []tiledProperty, name string, dst any) bool {
	for _, p := range props {
		if p.Name == name {
			return json.Unmarshal(p.Value, dst) == nil
		}
	}
	return false
}

// Parse reads one Tiled map.
func Parse(raw []byte) (*Map, error) {
	var tm tiledMap
	if err := json.Unmarshal(raw, &tm); err != nil {
		return nil, err
	}
	if tm.Infinite || tm.TileWidth != tm.TileHeight || tm.TileWidth <= 0 || tm.Width <= 0 || tm.Height <= 0 {
		return nil, errors.New("only finite orthogonal maps with square tiles are supported")
	}
	m := &Map{NPCs: map[string]NPC{}}
	if !prop(tm.Properties, "map_id", &m.ID) || !prop(tm.Properties, "map_version", &m.Version) {
		return nil, errors.New("map needs map_id and map_version properties")
	}
	prop(tm.Properties, "display_name", &m.DisplayName)

	tile := int32(tm.TileWidth) * sim.Unit
	w := &sim.World{
		Width: int32(tm.Width) * tile, Height: int32(tm.Height) * tile, TileSize: tile,
		Cols: int32(tm.Width), Rows: int32(tm.Height), Blocked: make([]bool, tm.Width*tm.Height),
	}
	var haveCollision, haveSpawn bool
	for _, l := range tm.Layers {
		switch {
		case l.Type == "tilelayer" && l.Name == "collision":
			if len(l.Data) != tm.Width*tm.Height {
				return nil, fmt.Errorf("collision layer has %d cells, want %d", len(l.Data), tm.Width*tm.Height)
			}
			for i, gid := range l.Data {
				w.Blocked[i] = gid != 0
			}
			haveCollision = true
		case l.Type == "objectgroup" && l.Name == "npcs":
			for _, o := range l.Objects {
				var code string
				if !prop(o.Properties, "npc_code", &code) || code == "" {
					return nil, fmt.Errorf("npc object %q has no npc_code", o.Name)
				}
				var radius float64
				if !prop(o.Properties, "interact_radius", &radius) || radius <= 0 {
					radius = 80
				}
				r := sim.Rect{
					X0: px(o.X), Y0: px(o.Y), X1: px(o.X + o.Width), Y1: px(o.Y + o.Height),
				}
				m.NPCs[code] = NPC{Code: code, X: (r.X0 + r.X1) / 2, Y: r.Y1, Collider: r, InteractRadius: px(radius)}
				w.Solids = append(w.Solids, r)
			}
		case l.Type == "objectgroup" && l.Name == "spawns":
			for _, o := range l.Objects {
				if o.Name == "default" {
					m.Spawn = [2]int32{px(o.X), px(o.Y)}
					haveSpawn = true
				}
			}
		}
	}
	if !haveCollision || !haveSpawn {
		return nil, errors.New("map needs a collision layer and a default spawn")
	}
	m.World = w
	if w.Collides(m.Spawn[0], m.Spawn[1]) {
		return nil, errors.New("default spawn collides")
	}
	if err := checkNPCs(m); err != nil {
		return nil, err
	}
	return m, nil
}

// checkNPCs enforces "NPCs never overlap each other or walls" (docs/02 §7),
// the publishing check the database cannot express.
func checkNPCs(m *Map) error {
	codes := make([]string, 0, len(m.NPCs))
	for c := range m.NPCs {
		codes = append(codes, c)
	}
	for i, a := range codes {
		ra := m.NPCs[a].Collider
		w := m.World
		for ty := ra.Y0 / w.TileSize; ty <= (ra.Y1-1)/w.TileSize; ty++ {
			for tx := ra.X0 / w.TileSize; tx <= (ra.X1-1)/w.TileSize; tx++ {
				if tx < 0 || ty < 0 || tx >= w.Cols || ty >= w.Rows || w.Blocked[ty*w.Cols+tx] {
					return fmt.Errorf("NPC %s overlaps a blocked tile", a)
				}
			}
		}
		for _, b := range codes[i+1:] {
			rb := m.NPCs[b].Collider
			if ra.X0 < rb.X1 && rb.X0 < ra.X1 && ra.Y0 < rb.Y1 && rb.Y0 < ra.Y1 {
				return fmt.Errorf("NPCs %s and %s overlap", a, b)
			}
		}
	}
	return nil
}

func px(v float64) int32 { return int32(v * sim.Unit) }

// Load reads maps.json and every map it lists from dir.
func Load(dir string) (*Registry, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "maps.json"))
	if err != nil {
		return nil, err
	}
	var reg struct {
		DefaultMap string `json:"default_map"`
		Maps       []struct {
			MapID  string `json:"map_id"`
			File   string `json:"file"`
			Public bool   `json:"public"`
		} `json:"maps"`
	}
	if err := json.Unmarshal(raw, &reg); err != nil {
		return nil, fmt.Errorf("maps.json: %w", err)
	}
	out := &Registry{Default: reg.DefaultMap, Maps: map[string]*Map{}}
	for _, e := range reg.Maps {
		raw, err := os.ReadFile(filepath.Join(dir, filepath.Base(e.File)))
		if err != nil {
			return nil, err
		}
		m, err := Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.File, err)
		}
		if m.ID != e.MapID {
			return nil, fmt.Errorf("%s declares map_id %q, registry says %q", e.File, m.ID, e.MapID)
		}
		m.Public = e.Public
		out.Maps[m.ID] = m
	}
	if _, ok := out.Maps[out.Default]; !ok {
		return nil, fmt.Errorf("default map %q not loaded", out.Default)
	}
	return out, nil
}
