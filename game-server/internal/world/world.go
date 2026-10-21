// Package world is the authoritative realtime world (docs/05 §4-5, docs/08):
// rooms of a public map with an Admin-configured capacity (P0-134), movement
// validated against the Tiled collision data (P0-137/138/168), the 30-second
// reconnect grace (P0-008/009/010/136) and public room chat (MVP-03).
//
// All state is owned by one goroutine (Run). Other goroutines talk to it by
// queueing closures, so there are no locks around game state.
package world

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"example.com/avatar/game-server/internal/gamedb"
	"example.com/avatar/game-server/internal/mapdata"
	"example.com/avatar/game-server/internal/proto"
	"example.com/avatar/game-server/internal/sim"
)

const ProtocolVersion = 1

// Close codes sent to clients (4000-4999 are application codes).
const (
	CloseLogout          = 4000
	CloseSessionReplaced = 4001
	CloseNotConfigured   = 4002
	CloseSlowConsumer    = 4008
	CloseProtocolError   = 4009
)

var (
	ErrNotConfigured = errors.New("world: room capacity is not configured")
	ErrNotJoined     = errors.New("world: connection has not joined")
	ErrRateLimited   = errors.New("world: rate limited")
	ErrRoomFull      = errors.New("world: room full")
	ErrClosed        = errors.New("world: stopped")
)

// Conn is one client connection. Implementations must be safe for concurrent
// use and must never block: a full buffer means the client is too slow.
type Conn interface {
	SendText(b []byte) bool
	SendBinary(b []byte) bool
	Close(code int, reason string)
}

// Config holds the technical parameters. Tick, snapshot rate and speed are
// development defaults to be confirmed by load tests ([CẦN TEST], GAP-15);
// the grace period is CHỐT (P0-008).
type Config struct {
	Tick            time.Duration
	SnapshotEvery   int
	SpeedPerStep    int32
	Grace           time.Duration
	MaxQueuedInputs int
	BurstTokens     int
	PersistEvery    time.Duration
}

func DefaultConfig() Config {
	return Config{Tick: 50 * time.Millisecond, SnapshotEvery: 2, SpeedPerStep: 800, Grace: 30 * time.Second,
		MaxQueuedInputs: 20, BurstTokens: 3, PersistEvery: 30 * time.Second}
}

// Identity is what the world needs to know about a joining character.
type Identity struct {
	CharacterID string
	AccountID   string
	DisplayName string
	MapID       string
	X, Y        int32
	HasPosition bool
	Appearance  gamedb.Appearance
}

type Player struct {
	Identity
	EntityID uint32
	room     *Room
	X, Y     int32
	Dir      uint8
	conn     Conn
	// Reconnect grace: while conn is nil the player stays in the room,
	// standing still and holding a slot, until graceUntil (P0-009).
	graceUntil time.Time

	inputs      []proto.MoveInput
	lastSeq     uint32
	ackSeq      uint32
	tokens      int
	movedRecent bool
	dirty       bool
	overflows   int
	lastChat    time.Time
}

type Room struct {
	ID      string
	Index   int
	Map     *mapdata.Map
	players map[uint32]*Player
	nextEnt uint32
}

type World struct {
	cfg     Config
	maps    *mapdata.Registry
	log     *slog.Logger
	persist func([]gamedb.Position)
	Now     func() time.Time

	queue     chan func()
	stopped   chan struct{}
	runtime   gamedb.Runtime
	rooms     map[string]*Room
	mapRooms  map[string][]*Room
	players   map[string]*Player // by character id
	byConn    map[Conn]*Player
	tickNo    uint32
	lastSaved time.Time
}

// New creates a world. persist receives position batches to store; it is
// called on its own goroutine and may block.
func New(cfg Config, maps *mapdata.Registry, persist func([]gamedb.Position), log *slog.Logger) *World {
	return &World{cfg: cfg, maps: maps, persist: persist, log: log, Now: time.Now,
		queue: make(chan func(), 4096), stopped: make(chan struct{}),
		rooms: map[string]*Room{}, mapRooms: map[string][]*Room{},
		players: map[string]*Player{}, byConn: map[Conn]*Player{}}
}

// Run owns the world until ctx ends, then saves every position.
func (w *World) Run(ctx context.Context) {
	t := time.NewTicker(w.cfg.Tick)
	defer t.Stop()
	w.lastSaved = w.Now()
	for {
		select {
		case fn := <-w.queue:
			fn()
		case <-t.C:
			w.tick()
		case <-ctx.Done():
			w.saveAll()
			close(w.stopped)
			return
		}
	}
}

func (w *World) do(fn func()) {
	select {
	case w.queue <- fn:
	case <-w.stopped:
	}
}

// call runs fn on the world goroutine and waits for it.
func (w *World) call(ctx context.Context, fn func()) error {
	done := make(chan struct{})
	select {
	case w.queue <- func() { fn(); close(done) }:
	case <-w.stopped:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-done:
		return nil
	case <-w.stopped:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Envelope is the JSON frame format of docs/08 §3.
type Envelope struct {
	V         int             `json:"v"`
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

func Message(typ, requestID string, payload any) []byte {
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	b, _ := json.Marshal(Envelope{V: ProtocolVersion, Type: typ, RequestID: requestID, Payload: raw})
	return b
}

type EntityInfo struct {
	EntityID    uint32            `json:"entity_id"`
	CharacterID string            `json:"character_id"`
	DisplayName string            `json:"display_name"`
	X           int32             `json:"x"`
	Y           int32             `json:"y"`
	Dir         uint8             `json:"dir"`
	Appearance  gamedb.Appearance `json:"appearance"`
}

func (p *Player) info() EntityInfo {
	return EntityInfo{EntityID: p.EntityID, CharacterID: p.CharacterID, DisplayName: p.DisplayName, X: p.X, Y: p.Y, Dir: p.Dir, Appearance: p.Appearance}
}

// SetAppearance installs a character's new outfit and shows it to everyone
// in its room after the change was committed (character.appearance.changed).
// An older version than the one known is ignored.
func (w *World) SetAppearance(characterID string, a gamedb.Appearance) {
	w.do(func() {
		p := w.players[characterID]
		if p == nil || a.Version < p.Appearance.Version {
			return
		}
		p.Appearance = a
		w.broadcast(p.room, Message("character.appearance.changed", "", map[string]any{
			"entity_id": p.EntityID, "character_id": p.CharacterID, "appearance": a}), nil)
	})
}

// SetDisplayName shows a committed rename (Rename Card) to the character's
// room; later chat and profiles read the name from the database.
func (w *World) SetDisplayName(characterID, name string) {
	w.do(func() {
		p := w.players[characterID]
		if p == nil {
			return
		}
		p.DisplayName = name
		w.broadcast(p.room, Message("character.renamed", "", map[string]any{
			"entity_id": p.EntityID, "character_id": p.CharacterID, "display_name": name}), nil)
	})
}

type Hello struct {
	ProtocolVersion int                `json:"protocol_version"`
	ServerTime      time.Time          `json:"server_time"`
	MapID           string             `json:"map_id"`
	MapVersion      string             `json:"map_version"`
	RoomID          string             `json:"room_id"`
	EntityID        uint32             `json:"entity_id"`
	ConfigVersion   int64              `json:"config_version"`
	GraceSeconds    int                `json:"grace_seconds"`
	TickMS          int                `json:"tick_ms"`
	SnapshotEvery   int                `json:"snapshot_every"`
	SpeedPerStep    int32              `json:"speed_per_step"`
	Resumed         bool               `json:"resumed"`
	ChatMaxChars    int                `json:"chat_max_chars"`
	Maintenance     gamedb.Maintenance `json:"maintenance"`
}

type RoomSnapshot struct {
	RoomID   string       `json:"room_id"`
	Entities []EntityInfo `json:"entities"`
}

// SetRuntime installs freshly loaded published config. A change of the
// maintenance switch is announced to everyone online (maintenance.notice).
func (w *World) SetRuntime(r gamedb.Runtime) {
	w.do(func() {
		changed := r.Maintenance != w.runtime.Maintenance
		w.runtime = r
		if changed {
			msg := Message("maintenance.notice", "", r.Maintenance)
			for _, room := range w.rooms {
				w.broadcast(room, msg, nil)
			}
		}
	})
}

// Join attaches a connection to a character. A character still in the room
// (connected elsewhere, or within the grace period) resumes in place: same
// room, same position, same slot (W-03). Otherwise it is placed at its saved
// position in a room with a free slot, a new room being opened when all are
// full (W-01, W-04).
func (w *World) Join(ctx context.Context, conn Conn, id Identity) (Hello, error) {
	var hello Hello
	var err error
	cerr := w.call(ctx, func() { hello, err = w.join(conn, id) })
	if cerr != nil {
		return hello, cerr
	}
	return hello, err
}

func (w *World) join(conn Conn, id Identity) (Hello, error) {
	p := w.players[id.CharacterID]
	resumed := p != nil
	if p != nil {
		if p.conn != nil && p.conn != conn {
			delete(w.byConn, p.conn)
			p.conn.Close(CloseSessionReplaced, "session_replaced")
		}
		p.conn = conn
		p.graceUntil = time.Time{}
		p.inputs, p.lastSeq, p.ackSeq, p.tokens = nil, 0, 0, w.cfg.BurstTokens
		if id.Appearance.Version > p.Appearance.Version {
			p.Appearance = id.Appearance
		}
	} else {
		m := w.maps.Maps[w.maps.Default]
		x, y := m.Spawn[0], m.Spawn[1]
		if id.HasPosition {
			if saved, ok := w.maps.Maps[id.MapID]; ok && saved.Public && !saved.World.Collides(id.X, id.Y) {
				m, x, y = saved, id.X, id.Y
			}
		}
		room, err := w.allocate(m)
		if err != nil {
			return Hello{}, err
		}
		p = &Player{Identity: id, X: x, Y: y, conn: conn, tokens: w.cfg.BurstTokens}
		w.addToRoom(p, room)
		w.players[id.CharacterID] = p
	}
	w.byConn[conn] = p
	hello := w.helloFor(p, resumed)
	conn.SendText(Message("hello", "", hello))
	conn.SendText(Message("room.snapshot", "", w.snapshotOf(p.room)))
	return hello, nil
}

func (w *World) helloFor(p *Player, resumed bool) Hello {
	return Hello{
		ProtocolVersion: ProtocolVersion, ServerTime: w.Now().UTC(), MapID: p.room.Map.ID, MapVersion: p.room.Map.Version,
		RoomID: p.room.ID, EntityID: p.EntityID, ConfigVersion: w.runtime.ConfigVersion,
		GraceSeconds: int(w.cfg.Grace / time.Second), TickMS: int(w.cfg.Tick / time.Millisecond),
		SnapshotEvery: w.cfg.SnapshotEvery, SpeedPerStep: w.cfg.SpeedPerStep, Resumed: resumed,
		ChatMaxChars: w.chatMaxChars(), Maintenance: w.runtime.Maintenance,
	}
}

func (w *World) allocate(m *mapdata.Map) (*Room, error) {
	capacity := w.runtime.RoomCapacity
	if capacity <= 0 {
		return nil, ErrNotConfigured
	}
	for _, r := range w.mapRooms[m.ID] {
		if len(r.players) < capacity {
			return r, nil
		}
	}
	// Reuse the lowest free room number so room ids stay short and stable.
	used := map[int]bool{}
	for _, r := range w.mapRooms[m.ID] {
		used[r.Index] = true
	}
	idx := 1
	for used[idx] {
		idx++
	}
	r := &Room{ID: fmt.Sprintf("%s#%d", m.ID, idx), Index: idx, Map: m, players: map[uint32]*Player{}}
	w.rooms[r.ID] = r
	w.mapRooms[m.ID] = append(w.mapRooms[m.ID], r)
	sort.Slice(w.mapRooms[m.ID], func(i, j int) bool { return w.mapRooms[m.ID][i].Index < w.mapRooms[m.ID][j].Index })
	return r, nil
}

func (w *World) addToRoom(p *Player, r *Room) {
	r.nextEnt++
	p.EntityID = r.nextEnt
	p.room = r
	r.players[p.EntityID] = p
	w.broadcast(r, Message("room.player_joined", "", p.info()), p)
}

func (w *World) snapshotOf(r *Room) RoomSnapshot {
	s := RoomSnapshot{RoomID: r.ID, Entities: []EntityInfo{}}
	for _, p := range r.sortedPlayers() {
		s.Entities = append(s.Entities, p.info())
	}
	return s
}

func (r *Room) sortedPlayers() []*Player {
	ps := make([]*Player, 0, len(r.players))
	for _, p := range r.players {
		ps = append(ps, p)
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].EntityID < ps[j].EntityID })
	return ps
}

// broadcast sends a JSON message to every connected member except skip.
func (w *World) broadcast(r *Room, msg []byte, skip *Player) {
	for _, p := range r.players {
		if p != skip && p.conn != nil {
			w.send(p, msg, false)
		}
	}
}

func (w *World) send(p *Player, msg []byte, binary bool) {
	var ok bool
	if binary {
		ok = p.conn.SendBinary(msg)
	} else {
		ok = p.conn.SendText(msg)
	}
	if !ok {
		w.dropConn(p, CloseSlowConsumer, "slow_consumer")
	}
}

// dropConn starts the grace period of a connected player.
func (w *World) dropConn(p *Player, code int, reason string) {
	if p.conn == nil {
		return
	}
	c := p.conn
	delete(w.byConn, c)
	p.conn = nil
	p.graceUntil = w.Now().Add(w.cfg.Grace)
	p.inputs = nil
	c.Close(code, reason)
}

// Disconnect is called when a connection ends. The character stays, standing
// still, for the grace period; nothing is broadcast (no "offline" icon).
func (w *World) Disconnect(conn Conn) {
	w.do(func() {
		if p := w.byConn[conn]; p != nil && p.conn == conn {
			delete(w.byConn, conn)
			p.conn = nil
			p.graceUntil = w.Now().Add(w.cfg.Grace)
			p.inputs = nil
		}
	})
}

// Input queues one movement input. Stale or duplicate sequence numbers are
// dropped; a client sending faster than the server applies inputs fills the
// queue and is logged for review, never banned automatically (P0-138, W-05).
func (w *World) Input(conn Conn, in proto.MoveInput) {
	w.do(func() {
		p := w.byConn[conn]
		if p == nil || in.Seq <= p.lastSeq {
			return
		}
		p.lastSeq = in.Seq
		if len(p.inputs) >= w.cfg.MaxQueuedInputs {
			p.overflows++
			if p.overflows == 1 || p.overflows%100 == 0 {
				w.log.Warn("movement input overflow: client sends faster than allowed",
					"character_id", p.CharacterID, "overflows", p.overflows, "review", p.overflows >= 100)
			}
			return
		}
		p.inputs = append(p.inputs, in)
	})
}

// CheckMapVersion answers client.ready: a client with another map version
// must reload; the server keeps validating with its own map (W-08).
func (w *World) CheckMapVersion(ctx context.Context, conn Conn, version string) (bool, string) {
	ok, current := true, ""
	_ = w.call(ctx, func() {
		if p := w.byConn[conn]; p != nil {
			current = p.room.Map.Version
			ok = version == current
		}
	})
	return ok, current
}

func (w *World) tick() {
	w.tickNo++
	now := w.Now()
	for _, r := range w.rooms {
		for _, p := range r.players {
			if p.conn == nil {
				continue
			}
			if p.tokens < w.cfg.BurstTokens {
				p.tokens++
			}
			for p.tokens > 0 && len(p.inputs) > 0 {
				in := p.inputs[0]
				p.inputs = p.inputs[1:]
				p.tokens--
				dx, dy := sim.ClampDir(in.DX, in.DY)
				nx, ny := r.Map.World.Step(p.X, p.Y, dx, dy, w.cfg.SpeedPerStep)
				if d, ok := sim.Dir8(dx, dy); ok {
					p.Dir = d
				}
				if nx != p.X || ny != p.Y {
					p.X, p.Y = nx, ny
					p.movedRecent, p.dirty = true, true
				}
				p.ackSeq = in.Seq
			}
		}
	}
	for _, p := range w.players {
		if p.conn == nil && !p.graceUntil.IsZero() && !now.Before(p.graceUntil) {
			w.remove(p, "timeout")
		}
	}
	if w.cfg.SnapshotEvery > 0 && w.tickNo%uint32(w.cfg.SnapshotEvery) == 0 {
		w.sendSnapshots()
	}
	if now.Sub(w.lastSaved) >= w.cfg.PersistEvery {
		w.lastSaved = now
		w.saveDirty()
	}
}

func (w *World) sendSnapshots() {
	for _, r := range w.rooms {
		ps := r.sortedPlayers()
		states := make([]proto.EntityState, 0, len(ps))
		for _, p := range ps {
			states = append(states, proto.EntityState{EntityID: p.EntityID, X: p.X, Y: p.Y, Dir: uint32(p.Dir), Moving: p.movedRecent})
		}
		ents := proto.EncodeEntities(states)
		for _, p := range ps {
			if p.conn != nil {
				w.send(p, proto.EncodeSnapshot(w.tickNo, p.ackSeq, ents), true)
			}
		}
		for _, p := range ps {
			p.movedRecent = false
		}
	}
}

// remove takes a character out of the world and saves its position: after
// the grace period the slot is free and a later login starts from the saved
// position (P0-010, W-04).
func (w *World) remove(p *Player, reason string) {
	if p.conn != nil {
		delete(w.byConn, p.conn)
		p.conn = nil
	}
	r := p.room
	delete(r.players, p.EntityID)
	delete(w.players, p.CharacterID)
	w.queuePersist([]gamedb.Position{{CharacterID: p.CharacterID, MapID: r.Map.ID, X: p.X, Y: p.Y}})
	w.broadcast(r, Message("room.player_left", "", map[string]any{"entity_id": p.EntityID, "reason": reason}), nil)
	if len(r.players) == 0 && r.Index != 1 {
		w.dropRoom(r)
	}
}

func (w *World) saveDirty() {
	var batch []gamedb.Position
	for _, p := range w.players {
		if p.dirty {
			batch = append(batch, gamedb.Position{CharacterID: p.CharacterID, MapID: p.room.Map.ID, X: p.X, Y: p.Y})
			p.dirty = false
		}
	}
	w.queuePersist(batch)
}

func (w *World) queuePersist(batch []gamedb.Position) {
	if len(batch) == 0 || w.persist == nil {
		return
	}
	go w.persist(batch)
}

func (w *World) saveAll() {
	var batch []gamedb.Position
	for _, p := range w.players {
		batch = append(batch, gamedb.Position{CharacterID: p.CharacterID, MapID: p.room.Map.ID, X: p.X, Y: p.Y})
	}
	if len(batch) > 0 && w.persist != nil {
		w.persist(batch)
	}
}

// KickAccount closes the account's connection. remove=false (a newer login)
// keeps the character for the grace period so the new session resumes it;
// remove=true (logout) takes it out of the world right away.
func (w *World) KickAccount(accountID string, code int, reason string, remove bool) {
	w.do(func() {
		for _, p := range w.players {
			if p.AccountID != accountID {
				continue
			}
			if remove {
				if p.conn != nil {
					p.conn.Close(code, reason)
				}
				w.remove(p, reason)
			} else {
				w.dropConn(p, code, reason)
			}
		}
	})
}

// Location is a character's authoritative position, used by the HTTP API to
// check NPC proximity.
type Location struct {
	MapID     string
	X, Y      int32
	Connected bool
}

func (w *World) Locate(ctx context.Context, characterID string) (Location, bool) {
	var loc Location
	var found bool
	_ = w.call(ctx, func() {
		if p := w.players[characterID]; p != nil {
			loc, found = Location{MapID: p.room.Map.ID, X: p.X, Y: p.Y, Connected: p.conn != nil}, true
		}
	})
	return loc, found
}

// Sender identifies the author of a chat message.
type Sender struct {
	CharacterID string `json:"character_id"`
	EntityID    uint32 `json:"entity_id"`
	DisplayName string `json:"display_name"`
	RoomID      string `json:"-"`
}

func (w *World) chatMaxChars() int {
	if w.runtime.ChatMaxChars > 0 {
		return w.runtime.ChatMaxChars
	}
	return 500 // technical safety bound while GAP-11 is open
}

func (w *World) chatMinInterval() time.Duration {
	if w.runtime.ChatMinIntervalMS > 0 {
		return time.Duration(w.runtime.ChatMinIntervalMS) * time.Millisecond
	}
	return 500 * time.Millisecond
}

// ChatPrecheck applies the per-character rate limit and returns the sender
// and the current chat limits.
func (w *World) ChatPrecheck(ctx context.Context, conn Conn) (Sender, int, error) {
	var s Sender
	var maxChars int
	var err error
	cerr := w.call(ctx, func() {
		p := w.byConn[conn]
		if p == nil {
			err = ErrNotJoined
			return
		}
		now := w.Now()
		if now.Sub(p.lastChat) < w.chatMinInterval() {
			err = ErrRateLimited
			return
		}
		p.lastChat = now
		maxChars = w.chatMaxChars()
		s = Sender{CharacterID: p.CharacterID, EntityID: p.EntityID, DisplayName: p.DisplayName, RoomID: p.room.ID}
	})
	if cerr != nil {
		return s, 0, cerr
	}
	return s, maxChars, err
}

// BroadcastRoom sends a JSON message to every connected member of a room
// (public chat goes to the room only, never server-wide; S-01).
func (w *World) BroadcastRoom(roomID string, msg []byte) {
	w.do(func() {
		if r := w.rooms[roomID]; r != nil {
			w.broadcast(r, msg, nil)
		}
	})
}

// SendTo delivers a JSON message to a character if it is connected now.
func (w *World) SendTo(characterID string, msg []byte) {
	w.do(func() {
		if p := w.players[characterID]; p != nil && p.conn != nil {
			w.send(p, msg, false)
		}
	})
}

// Presence is what friends may see of a character.
type Presence struct {
	Online bool   `json:"online"`
	RoomID string `json:"room_id,omitempty"`
}

// PresenceOf reports, for each character id, whether it is connected and in
// which room. A character inside its reconnect grace counts as online: it
// still stands in the room (P0-009).
func (w *World) PresenceOf(ctx context.Context, ids []string) map[string]Presence {
	out := map[string]Presence{}
	_ = w.call(ctx, func() {
		for _, id := range ids {
			if p := w.players[id]; p != nil {
				out[id] = Presence{Online: true, RoomID: p.room.ID}
			}
		}
	})
	return out
}

var (
	ErrFriendOffline = errors.New("world: friend is not in the world")
	ErrSameRoom      = errors.New("world: already in that room")
)

// JoinFriendRoom moves a connected character into its friend's room, next
// to the friend, when that room has a free slot (P0-135). A full room is
// refused and the character stays where it is (W-02). Friendship is checked
// by the caller.
func (w *World) JoinFriendRoom(ctx context.Context, characterID, friendID string) (string, error) {
	var roomID string
	var err error
	cerr := w.call(ctx, func() {
		p, f := w.players[characterID], w.players[friendID]
		switch {
		case p == nil || p.conn == nil:
			err = ErrNotJoined
			return
		case f == nil:
			err = ErrFriendOffline
			return
		case p.room == f.room:
			err = ErrSameRoom
			return
		case !f.room.Map.Public:
			err = ErrFriendOffline
			return
		case len(f.room.players) >= w.runtime.RoomCapacity:
			err = ErrRoomFull
			return
		}
		old := p.room
		delete(old.players, p.EntityID)
		w.broadcast(old, Message("room.player_left", "", map[string]any{"entity_id": p.EntityID, "reason": "moved"}), nil)
		if len(old.players) == 0 && old.Index != 1 {
			w.dropRoom(old)
		}
		p.X, p.Y = w.spotNear(f.room.Map, f.X, f.Y)
		p.inputs, p.lastSeq, p.ackSeq, p.tokens, p.dirty = nil, 0, 0, w.cfg.BurstTokens, true
		w.addToRoom(p, f.room)
		roomID = f.room.ID
		hello := w.helloFor(p, false)
		p.conn.SendText(Message("hello", "", hello))
		p.conn.SendText(Message("room.snapshot", "", w.snapshotOf(p.room)))
	})
	if cerr != nil {
		return "", cerr
	}
	return roomID, err
}

// spotNear finds a free standing spot a step away from (x, y).
func (w *World) spotNear(m *mapdata.Map, x, y int32) (int32, int32) {
	for _, d := range [][2]int32{{0, 3200}, {-3200, 0}, {3200, 0}, {0, -3200}, {2400, 2400}, {-2400, 2400}} {
		if !m.World.Collides(x+d[0], y+d[1]) {
			return x + d[0], y + d[1]
		}
	}
	return x, y
}

func (w *World) dropRoom(r *Room) {
	delete(w.rooms, r.ID)
	list := w.mapRooms[r.Map.ID]
	for i, x := range list {
		if x == r {
			w.mapRooms[r.Map.ID] = append(list[:i], list[i+1:]...)
			break
		}
	}
}

// Stats is for tests and monitoring.
type Stats struct {
	Rooms map[string]int
	Grace int
}

func (w *World) Stats(ctx context.Context) Stats {
	s := Stats{Rooms: map[string]int{}}
	_ = w.call(ctx, func() {
		for id, r := range w.rooms {
			s.Rooms[id] = len(r.players)
		}
		for _, p := range w.players {
			if p.conn == nil {
				s.Grace++
			}
		}
	})
	return s
}
