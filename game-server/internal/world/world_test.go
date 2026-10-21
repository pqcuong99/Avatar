package world

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/avatar/game-server/internal/gamedb"
	"example.com/avatar/game-server/internal/mapdata"
	"example.com/avatar/game-server/internal/proto"
)

type fakeConn struct {
	mu     sync.Mutex
	texts  []Envelope
	frames []proto.Snapshot
	closed int
}

func (c *fakeConn) SendText(b []byte) bool {
	var e Envelope
	_ = json.Unmarshal(b, &e)
	c.mu.Lock()
	c.texts = append(c.texts, e)
	c.mu.Unlock()
	return true
}

func (c *fakeConn) SendBinary(b []byte) bool {
	s, _ := proto.DecodeSnapshot(b)
	c.mu.Lock()
	c.frames = append(c.frames, s)
	c.mu.Unlock()
	return true
}

func (c *fakeConn) Close(code int, _ string) { c.mu.Lock(); c.closed = code; c.mu.Unlock() }

func (c *fakeConn) last(typ string) *Envelope {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.texts) - 1; i >= 0; i-- {
		if c.texts[i].Type == typ {
			e := c.texts[i]
			return &e
		}
	}
	return nil
}

func (c *fakeConn) lastFrame() proto.Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.frames) == 0 {
		return proto.Snapshot{}
	}
	return c.frames[len(c.frames)-1]
}

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time          { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) advance(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

type harness struct {
	t     *testing.T
	w     *World
	clock *clock
	saved chan []gamedb.Position
	m     *mapdata.Map
}

func newHarness(t *testing.T, capacity int) *harness {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "shared", "maps", "maps.json")); err == nil {
			break
		}
		dir = filepath.Dir(dir)
	}
	reg, err := mapdata.Load(filepath.Join(dir, "shared", "maps"))
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, clock: &clock{t: time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC)}, saved: make(chan []gamedb.Position, 100), m: reg.Maps["plaza"]}
	cfg := DefaultConfig()
	cfg.Tick = time.Hour // ticks are driven by the test
	h.w = New(cfg, reg, func(p []gamedb.Position) { h.saved <- p }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h.w.Now = h.clock.now
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go h.w.Run(ctx)
	h.w.SetRuntime(gamedb.Runtime{ConfigVersion: 1, RoomCapacity: capacity})
	return h
}

// tick runs n world ticks on the world goroutine.
func (h *harness) tick(n int) {
	for i := 0; i < n; i++ {
		_ = h.w.call(context.Background(), h.w.tick)
	}
}

func (h *harness) join(id string, conn *fakeConn) Hello {
	h.t.Helper()
	hello, err := h.w.Join(context.Background(), conn, Identity{CharacterID: id, AccountID: "acc-" + id, DisplayName: "P " + id})
	if err != nil {
		h.t.Fatalf("join %s: %v", id, err)
	}
	return hello
}

func TestW01RoomsNeverExceedCapacity(t *testing.T) {
	h := newHarness(t, 3)
	rooms := map[string]int{}
	for i := 0; i < 7; i++ {
		hello := h.join(string(rune('a'+i)), &fakeConn{})
		rooms[hello.RoomID]++
	}
	if rooms["plaza#1"] != 3 || rooms["plaza#2"] != 3 || rooms["plaza#3"] != 1 {
		t.Fatalf("rooms %v", rooms)
	}
}

func TestJoinWithoutConfiguredCapacityIsRefused(t *testing.T) {
	h := newHarness(t, 0)
	if _, err := h.w.Join(context.Background(), &fakeConn{}, Identity{CharacterID: "x"}); err != ErrNotConfigured {
		t.Fatalf("got %v", err)
	}
}

func TestW03ReconnectWithinGraceKeepsRoomPositionAndSlot(t *testing.T) {
	h := newHarness(t, 2)
	c1 := &fakeConn{}
	first := h.join("a", c1)
	h.join("b", &fakeConn{})
	// Move a little so the position is not the spawn.
	for i := uint32(1); i <= 5; i++ {
		h.w.Input(c1, proto.MoveInput{Seq: i, DX: 1000})
	}
	h.tick(10)
	loc, _ := h.w.Locate(context.Background(), "a")

	h.w.Disconnect(c1)
	h.w.Stats(context.Background()) // wait until the disconnect is processed
	h.clock.advance(29 * time.Second)
	h.tick(1)
	// The room is full: a's slot is still held, so a newcomer goes elsewhere.
	if hello := h.join("c", &fakeConn{}); hello.RoomID == first.RoomID {
		t.Fatal("newcomer took the slot held during grace")
	}
	c2 := &fakeConn{}
	again := h.join("a", c2)
	if !again.Resumed || again.RoomID != first.RoomID || again.EntityID != first.EntityID {
		t.Fatalf("resume %+v vs %+v", again, first)
	}
	loc2, _ := h.w.Locate(context.Background(), "a")
	if loc2.X != loc.X || loc2.Y != loc.Y {
		t.Fatalf("position changed: %+v -> %+v", loc, loc2)
	}
	if st := h.w.Stats(context.Background()); st.Rooms[first.RoomID] != 2 {
		t.Fatalf("stats %+v", st)
	}
}

func TestW04AfterGraceTheSlotIsFreedAndPositionSaved(t *testing.T) {
	h := newHarness(t, 2)
	c1 := &fakeConn{}
	first := h.join("a", c1)
	other := &fakeConn{}
	h.join("b", other)
	for i := uint32(1); i <= 3; i++ {
		h.w.Input(c1, proto.MoveInput{Seq: i, DY: 1000})
	}
	h.tick(5)
	loc, _ := h.w.Locate(context.Background(), "a")
	h.w.Disconnect(c1)
	h.w.Stats(context.Background()) // wait until the disconnect is processed
	h.clock.advance(31 * time.Second)
	h.tick(1)
	if _, found := h.w.Locate(context.Background(), "a"); found {
		t.Fatal("still in world after grace")
	}
	saved := <-h.saved
	if len(saved) != 1 || saved[0].CharacterID != "a" || saved[0].X != loc.X || saved[0].Y != loc.Y {
		t.Fatalf("saved %+v want %+v", saved, loc)
	}
	if left := other.last("room.player_left"); left == nil {
		t.Fatal("others not told that a left")
	}
	if st := h.w.Stats(context.Background()); st.Rooms[first.RoomID] != 1 {
		t.Fatalf("slot not freed: %+v", st)
	}
	// Coming back later: a fresh join at the saved position.
	again, err := h.w.Join(context.Background(), &fakeConn{}, Identity{CharacterID: "a", MapID: "plaza", X: loc.X, Y: loc.Y, HasPosition: true})
	if err != nil || again.Resumed {
		t.Fatalf("rejoin %+v %v", again, err)
	}
	loc2, _ := h.w.Locate(context.Background(), "a")
	if loc2.X != loc.X || loc2.Y != loc.Y {
		t.Fatalf("not placed at saved position: %+v", loc2)
	}
}

func TestSavedPositionInsideAWallFallsBackToSpawn(t *testing.T) {
	h := newHarness(t, 5)
	_, err := h.w.Join(context.Background(), &fakeConn{}, Identity{CharacterID: "a", MapID: "plaza", X: 100, Y: 100, HasPosition: true})
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := h.w.Locate(context.Background(), "a")
	if loc.X != h.m.Spawn[0] || loc.Y != h.m.Spawn[1] {
		t.Fatalf("got %+v", loc)
	}
}

func TestW05FloodedInputsCannotOutrunTheServer(t *testing.T) {
	h := newHarness(t, 5)
	c := &fakeConn{}
	h.join("a", c)
	start, _ := h.w.Locate(context.Background(), "a")
	// 200 inputs at once: a speed hack attempt.
	for i := uint32(1); i <= 200; i++ {
		h.w.Input(c, proto.MoveInput{Seq: i, DX: -1000})
	}
	h.tick(4)
	loc, _ := h.w.Locate(context.Background(), "a")
	// At most BurstTokens + ticks steps of 800 units.
	if moved := start.X - loc.X; moved > int32(3+4)*800 {
		t.Fatalf("moved %d units in 4 ticks", moved)
	}
	// Replayed or stale sequence numbers are ignored.
	h.w.Input(c, proto.MoveInput{Seq: 5, DX: 1000})
	h.tick(30)
	final, _ := h.w.Locate(context.Background(), "a")
	if final.X > loc.X {
		t.Fatal("stale input applied")
	}
}

func TestSnapshotsAckInputsAndShowOthers(t *testing.T) {
	h := newHarness(t, 5)
	a, b := &fakeConn{}, &fakeConn{}
	helloA := h.join("a", a)
	helloB := h.join("b", b)
	if j := a.last("room.player_joined"); j == nil {
		t.Fatal("a not told that b joined")
	}
	h.w.Input(a, proto.MoveInput{Seq: 1, DX: 1000})
	h.w.Input(a, proto.MoveInput{Seq: 2, DX: 1000})
	h.tick(2)
	fa, fb := a.lastFrame(), b.lastFrame()
	if fa.AckSeq != 2 || fb.AckSeq != 0 {
		t.Fatalf("acks a=%d b=%d", fa.AckSeq, fb.AckSeq)
	}
	if len(fb.Entities) != 2 {
		t.Fatalf("b sees %d entities", len(fb.Entities))
	}
	var aState proto.EntityState
	for _, e := range fb.Entities {
		if e.EntityID == helloA.EntityID {
			aState = e
		}
	}
	if aState.X != h.m.Spawn[0]+1600 || !aState.Moving || aState.Dir != 6 {
		t.Fatalf("a as seen by b: %+v", aState)
	}
	_ = helloB
}

func TestS01PublicChatStaysInTheRoom(t *testing.T) {
	h := newHarness(t, 1)
	a, b := &fakeConn{}, &fakeConn{}
	ha := h.join("a", a)
	hb := h.join("b", b)
	if ha.RoomID == hb.RoomID {
		t.Fatal("expected two rooms")
	}
	sender, _, err := h.w.ChatPrecheck(context.Background(), a)
	if err != nil || sender.RoomID != ha.RoomID {
		t.Fatalf("precheck %+v %v", sender, err)
	}
	h.w.BroadcastRoom(sender.RoomID, Message("chat.public.delivered", "r1", map[string]string{"body": "hi"}))
	h.tick(1)
	if a.last("chat.public.delivered") == nil || b.last("chat.public.delivered") != nil {
		t.Fatal("chat leaked across rooms or not delivered")
	}
	if _, _, err := h.w.ChatPrecheck(context.Background(), a); err != ErrRateLimited {
		t.Fatalf("second message right away: %v", err)
	}
}

func TestW02JoinFriendRoomRespectsCapacity(t *testing.T) {
	h := newHarness(t, 2)
	friend := &fakeConn{}
	hf := h.join("friend", friend)
	h.join("other", &fakeConn{}) // friend's room is now full
	me := &fakeConn{}
	hm := h.join("me", me)
	if hm.RoomID == hf.RoomID {
		t.Fatal("setup: expected different rooms")
	}
	if _, err := h.w.JoinFriendRoom(context.Background(), "me", "friend"); err != ErrRoomFull {
		t.Fatalf("full room: %v", err)
	}
	if loc, _ := h.w.Locate(context.Background(), "me"); loc.MapID == "" {
		t.Fatal("player lost its place")
	}
	if st := h.w.Stats(context.Background()); st.Rooms[hm.RoomID] != 1 || st.Rooms[hf.RoomID] != 2 {
		t.Fatalf("rooms changed: %+v", st)
	}
	// A slot frees up: the move works and the client gets a fresh hello.
	h.w.KickAccount("acc-other", CloseLogout, "logout", true)
	room, err := h.w.JoinFriendRoom(context.Background(), "me", "friend")
	if err != nil || room != hf.RoomID {
		t.Fatalf("join: %s %v", room, err)
	}
	if hello := me.last("hello"); hello == nil {
		t.Fatal("no hello after the move")
	}
	if _, err := h.w.JoinFriendRoom(context.Background(), "me", "friend"); err != ErrSameRoom {
		t.Fatalf("same room: %v", err)
	}
	if _, err := h.w.JoinFriendRoom(context.Background(), "me", "nobody"); err != ErrFriendOffline {
		t.Fatalf("offline friend: %v", err)
	}
}

func TestAppearanceChangesReachTheRoomInVersionOrder(t *testing.T) {
	h := newHarness(t, 5)
	a, b := &fakeConn{}, &fakeConn{}
	h.join("a", a)
	h.join("b", b)
	look := gamedb.Appearance{Version: 2, Items: map[string]gamedb.AppearanceItem{"hat": {ItemCode: "hat_straw"}}}
	h.w.SetAppearance("a", look)
	h.w.SetAppearance("a", gamedb.Appearance{Version: 1}) // stale: ignored
	h.tick(1)
	got := b.last("character.appearance.changed")
	if got == nil || !strings.Contains(string(got.Payload), "hat_straw") {
		t.Fatalf("b did not see the new outfit: %+v", got)
	}
	// A newcomer gets the current outfit in its room snapshot.
	c := &fakeConn{}
	h.join("c", c)
	if snap := c.last("room.snapshot"); snap == nil || !strings.Contains(string(snap.Payload), "hat_straw") {
		t.Fatal("snapshot without appearance")
	}
}

func TestNewLoginKicksTheOldConnectionButKeepsTheCharacter(t *testing.T) {
	h := newHarness(t, 5)
	old := &fakeConn{}
	h.join("a", old)
	h.w.KickAccount("acc-a", CloseSessionReplaced, "session_replaced", false)
	h.tick(1)
	if old.closed != CloseSessionReplaced {
		t.Fatalf("old connection closed with %d", old.closed)
	}
	if st := h.w.Stats(context.Background()); st.Grace != 1 {
		t.Fatalf("stats %+v", st)
	}
	fresh := &fakeConn{}
	if hello := h.join("a", fresh); !hello.Resumed {
		t.Fatal("new session did not resume")
	}
	// Logout removes right away.
	h.w.KickAccount("acc-a", CloseLogout, "logout", true)
	if _, found := h.w.Locate(context.Background(), "a"); found {
		t.Fatal("still in world after logout")
	}
}

func TestRenameIsShownToTheRoomAndUsedForChat(t *testing.T) {
	h := newHarness(t, 5)
	a, b := &fakeConn{}, &fakeConn{}
	h.join("a", a)
	h.join("b", b)
	h.w.SetDisplayName("a", "Tên Mới")
	h.tick(1)
	if got := b.last("character.renamed"); got == nil || !strings.Contains(string(got.Payload), "Tên Mới") {
		t.Fatalf("b did not see the rename: %+v", got)
	}
	c := &fakeConn{}
	h.join("c", c)
	if snap := c.last("room.snapshot"); snap == nil || !strings.Contains(string(snap.Payload), "Tên Mới") {
		t.Fatal("snapshot with the old name")
	}
}
