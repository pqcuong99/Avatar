package proto

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Golden frames shared with client/src/game/net/proto.test.ts. Regenerate with
// UPDATE_GOLDEN=1 after an intended protocol change (and bump the protocol).
type goldenFrames struct {
	Client []struct {
		Seq uint32 `json:"seq"`
		DX  int32  `json:"dx"`
		DY  int32  `json:"dy"`
		Hex string `json:"hex"`
	} `json:"client"`
	Server []struct {
		ServerTick uint32        `json:"server_tick"`
		AckSeq     uint32        `json:"ack_seq"`
		Entities   []EntityState `json:"entities"`
		Hex        string        `json:"hex"`
	} `json:"server"`
}

func fixturePath(t *testing.T) string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "shared", "proto")); err == nil {
			return filepath.Join(dir, "shared", "fixtures", "proto_frames.json")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("shared/proto not found")
		}
		dir = parent
	}
}

func TestGoldenFrames(t *testing.T) {
	var g goldenFrames
	inputs := []MoveInput{{Seq: 1, DX: 707, DY: -707}, {Seq: 4294967295, DX: -1000, DY: 0}, {Seq: 2}}
	snaps := []Snapshot{
		{ServerTick: 12345, AckSeq: 77, Entities: []EntityState{{EntityID: 1, X: 75200, Y: 60800, Dir: 4, Moving: true}, {EntityID: 2, X: -5, Y: 0}}},
		{ServerTick: 1},
	}
	for _, in := range inputs {
		g.Client = append(g.Client, struct {
			Seq uint32 `json:"seq"`
			DX  int32  `json:"dx"`
			DY  int32  `json:"dy"`
			Hex string `json:"hex"`
		}{in.Seq, in.DX, in.DY, hex.EncodeToString(EncodeClientFrame(in))})
	}
	for _, s := range snaps {
		g.Server = append(g.Server, struct {
			ServerTick uint32        `json:"server_tick"`
			AckSeq     uint32        `json:"ack_seq"`
			Entities   []EntityState `json:"entities"`
			Hex        string        `json:"hex"`
		}{s.ServerTick, s.AckSeq, s.Entities, hex.EncodeToString(EncodeSnapshot(s.ServerTick, s.AckSeq, EncodeEntities(s.Entities)))})
	}
	path := fixturePath(t)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		raw, _ := json.MarshalIndent(g, "", "  ")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with UPDATE_GOLDEN=1 to create it)", err)
	}
	var want goldenFrames
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, g) {
		t.Fatalf("encodings changed:\nwant %+v\ngot  %+v", want, g)
	}
	for i, c := range want.Client {
		b, _ := hex.DecodeString(c.Hex)
		in, ok, err := DecodeClientFrame(b)
		if err != nil || !ok || in != inputs[i] {
			t.Fatalf("decode client %d: %+v %v %v", i, in, ok, err)
		}
	}
	for i, s := range want.Server {
		b, _ := hex.DecodeString(s.Hex)
		got, err := DecodeSnapshot(b)
		if err != nil || got.ServerTick != snaps[i].ServerTick || got.AckSeq != snaps[i].AckSeq || !reflect.DeepEqual(got.Entities, snaps[i].Entities) {
			t.Fatalf("decode server %d: %+v %v", i, got, err)
		}
	}
}

func TestDecodeRejectsGarbage(t *testing.T) {
	if _, _, err := DecodeClientFrame([]byte{0x0a, 0xff}); err == nil {
		t.Fatal("truncated frame accepted")
	}
	if _, ok, err := DecodeClientFrame([]byte{0x10, 0x01}); err != nil || ok {
		t.Fatal("frame without move must decode as empty")
	}
}
