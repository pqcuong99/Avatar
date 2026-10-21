package realtime_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"example.com/project-thanh/game-server/internal/api"
	"example.com/project-thanh/game-server/internal/authn"
	"example.com/project-thanh/game-server/internal/gamedb"
	"example.com/project-thanh/game-server/internal/mapdata"
	"example.com/project-thanh/game-server/internal/proto"
	"example.com/project-thanh/game-server/internal/realtime"
	"example.com/project-thanh/game-server/internal/testenv"
	"example.com/project-thanh/game-server/internal/txclient"
	"example.com/project-thanh/game-server/internal/world"
)

const origin = "http://game.test"

type stack struct {
	t      *testing.T
	pool   *pgxpool.Pool
	srv    *httptest.Server
	world  *world.World
	maps   *mapdata.Registry
	txHits chan string
}

func newStack(t *testing.T) *stack {
	t.Helper()
	pool := testenv.DB(t)
	ctx := context.Background()
	for _, sql := range []string{
		`INSERT INTO game_config_versions (id, label, state) VALUES (1, 'T', 'draft')`,
		`INSERT INTO game_config_values VALUES (1, 'world.room_capacity', '10'), (1, 'chat.public', '{"max_chars": 50, "min_interval_ms": 1}')`,
		`UPDATE game_config_versions SET state = 'published', published_at = now() WHERE id = 1`,
	} {
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	maps, err := mapdata.Load(testenv.RepoPath(t, "shared", "maps"))
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	db := &gamedb.Store{Pool: pool}
	w := world.New(world.DefaultConfig(), maps, func(ps []gamedb.Position) { _ = db.SavePositions(context.Background(), ps) }, log)
	wctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go w.Run(wctx)
	rt, err := db.LoadRuntime(ctx)
	if err != nil || rt.RoomCapacity != 10 || rt.ChatMaxChars != 50 {
		t.Fatalf("runtime %+v %v", rt, err)
	}
	w.SetRuntime(rt)

	s := &stack{t: t, pool: pool, world: w, maps: maps, txHits: make(chan string, 10)}
	fakeTx := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		s.txHits <- r.URL.Path
		rw.Header().Set("Content-Type", "application/json")
		_, _ = rw.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	t.Cleanup(fakeTx.Close)

	auth := authn.NewStore(pool)
	auth.BcryptCost = bcrypt.MinCost
	a := &api.API{Auth: auth, Limiter: authn.NewLimiter(), RegisterLimit: authn.NewLimiter(), DB: db,
		Tx: txclient.New(fakeTx.URL, "test-token-0123456789"), World: w, Maps: maps, Log: log,
		CookieName: "thanh_session", AllowedOrigins: []string{origin}}
	mux := http.NewServeMux()
	a.Routes(mux)
	mux.Handle("GET /ws", &realtime.Handler{World: w, Auth: auth, DB: db, CookieName: "thanh_session",
		OriginPatterns: []string{"game.test"}, Log: log})
	s.srv = httptest.NewServer(mux)
	t.Cleanup(s.srv.Close)
	return s
}

func (s *stack) post(path, cookie string, body any) (int, map[string]any, *http.Response) {
	s.t.Helper()
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, s.srv.URL+path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", origin)
	if cookie != "" {
		req.Header.Set("Cookie", "thanh_session="+cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out, resp
}

// player registers, logs in and gets a character; returns the session token.
func (s *stack) player(name string) string {
	s.t.Helper()
	if code, out, _ := s.post("/api/v1/auth/register", "", map[string]string{"username": name, "password": "password-123"}); code != 200 {
		s.t.Fatalf("register %s: %d %v", name, code, out)
	}
	token := s.login(name)
	var accountID string
	code, out, _ := s.get("/api/v1/auth/me", token)
	if code != 200 {
		s.t.Fatalf("me: %d %v", code, out)
	}
	accountID = out["result"].(map[string]any)["account"].(map[string]any)["id"].(string)
	testenv.Character(s.t, s.pool, accountID, "Nhân vật "+name)
	return token
}

func (s *stack) login(name string) string {
	s.t.Helper()
	code, out, resp := s.post("/api/v1/auth/login", "", map[string]string{"username": name, "password": "password-123"})
	if code != 200 {
		s.t.Fatalf("login %s: %d %v", name, code, out)
	}
	for _, c := range resp.Cookies() {
		if c.Name == "thanh_session" {
			if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
				s.t.Fatal("session cookie must be HttpOnly and SameSite=Lax")
			}
			return c.Value
		}
	}
	s.t.Fatal("no session cookie")
	return ""
}

func (s *stack) get(path, cookie string) (int, map[string]any, *http.Response) {
	req, _ := http.NewRequest(http.MethodGet, s.srv.URL+path, nil)
	req.Header.Set("Cookie", "thanh_session="+cookie)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out, resp
}

type client struct {
	t     *testing.T
	ws    *websocket.Conn
	hello world.Hello
}

func (s *stack) connect(token string) *client {
	s.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h := http.Header{}
	h.Set("Cookie", "thanh_session="+token)
	h.Set("Origin", origin)
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(s.srv.URL, "http")+"/ws", &websocket.DialOptions{HTTPHeader: h})
	if err != nil {
		s.t.Fatalf("dial: %v", err)
	}
	s.t.Cleanup(func() { _ = ws.CloseNow() })
	c := &client{t: s.t, ws: ws}
	env := c.until("hello", nil)
	_ = json.Unmarshal(env.Payload, &c.hello)
	c.until("room.snapshot", nil)
	return c
}

// until reads text frames (skipping snapshots) until one of type typ that
// satisfies ok arrives.
func (c *client) until(typ string, ok func(world.Envelope) bool) world.Envelope {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		mt, data, err := c.ws.Read(ctx)
		if err != nil {
			c.t.Fatalf("waiting for %s: %v", typ, err)
		}
		if mt != websocket.MessageText {
			continue
		}
		var env world.Envelope
		_ = json.Unmarshal(data, &env)
		if env.Type == typ && (ok == nil || ok(env)) {
			return env
		}
	}
}

func (c *client) snapshotUntil(ok func(proto.Snapshot) bool) proto.Snapshot {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		mt, data, err := c.ws.Read(ctx)
		if err != nil {
			c.t.Fatalf("waiting for snapshot: %v", err)
		}
		if mt != websocket.MessageBinary {
			continue
		}
		s, err := proto.DecodeSnapshot(data)
		if err != nil {
			c.t.Fatal(err)
		}
		if ok(s) {
			return s
		}
	}
}

func (c *client) sendText(typ, requestID string, payload any) {
	raw, _ := json.Marshal(payload)
	b, _ := json.Marshal(world.Envelope{V: 1, Type: typ, RequestID: requestID, Payload: raw})
	if err := c.ws.Write(context.Background(), websocket.MessageText, b); err != nil {
		c.t.Fatal(err)
	}
}

func TestTwoPlayersSeeEachOtherMoveAndChat(t *testing.T) {
	s := newStack(t)
	alice := s.connect(s.player("alice"))
	bob := s.connect(s.player("bobby"))
	if alice.hello.RoomID != bob.hello.RoomID || alice.hello.MapVersion != s.maps.Maps["plaza"].Version {
		t.Fatalf("hello %+v / %+v", alice.hello, bob.hello)
	}
	alice.until("room.player_joined", nil)

	// Alice walks east 5 steps; Bob sees it in a binary snapshot (P0-137).
	for i := uint32(1); i <= 5; i++ {
		_ = alice.ws.Write(context.Background(), websocket.MessageBinary, proto.EncodeClientFrame(proto.MoveInput{Seq: i, DX: 1000}))
	}
	spawnX := s.maps.Maps["plaza"].Spawn[0]
	bob.snapshotUntil(func(sn proto.Snapshot) bool {
		for _, e := range sn.Entities {
			if e.EntityID == alice.hello.EntityID && e.X == spawnX+5*800 {
				return true
			}
		}
		return false
	})
	alice.snapshotUntil(func(sn proto.Snapshot) bool { return sn.AckSeq == 5 })

	// Public chat reaches the room; a retry with the same request_id is not
	// broadcast again (S-10).
	alice.sendText("chat.public.send", "chat-req-0001", map[string]string{"body": "  Xin chào <b>mọi người</b>  "})
	got := bob.until("chat.public.delivered", nil)
	var p struct {
		Body   string `json:"body"`
		Sender struct {
			DisplayName string `json:"display_name"`
		} `json:"sender"`
	}
	_ = json.Unmarshal(got.Payload, &p)
	if p.Body != "Xin chào <b>mọi người</b>" || p.Sender.DisplayName != "Nhân vật alice" {
		t.Fatalf("chat %+v", p)
	}
	alice.until("chat.public.delivered", nil)
	time.Sleep(5 * time.Millisecond)
	alice.sendText("chat.public.send", "chat-req-0001", map[string]string{"body": "Xin chào <b>mọi người</b>"})
	alice.until("chat.public.delivered", func(e world.Envelope) bool { return e.RequestID == "chat-req-0001" })
	time.Sleep(5 * time.Millisecond)
	alice.sendText("chat.public.send", "chat-req-0002", map[string]string{"body": strings.Repeat("x", 51)})
	alice.until("chat.public.failed", nil)
	time.Sleep(5 * time.Millisecond) // rejected messages count towards the rate limit too
	alice.sendText("chat.public.send", "chat-req-0003", map[string]string{"body": "second"})
	second := bob.until("chat.public.delivered", nil)
	if second.RequestID != "chat-req-0003" {
		t.Fatalf("bob got the duplicate %s", second.RequestID)
	}

	// W-08: a client with another map version is told to reload.
	bob.sendText("client.ready", "ready-00001", map[string]string{"map_version": "plaza@0"})
	bob.until("error", func(e world.Envelope) bool { return strings.Contains(string(e.Payload), "MAP_VERSION_MISMATCH") })
}

func TestA01LoginElsewhereClosesTheOldSocket(t *testing.T) {
	s := newStack(t)
	token := s.player("carol")
	old := s.connect(token)
	newToken := s.login("carol")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		_, _, err := old.ws.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) != world.CloseSessionReplaced {
				t.Fatalf("closed with %v", err)
			}
			break
		}
	}
	if code, _, _ := s.get("/api/v1/state", token); code != http.StatusUnauthorized {
		t.Fatalf("old cookie still works: %d", code)
	}
	fresh := s.connect(newToken)
	if !fresh.hello.Resumed {
		t.Fatal("new session did not resume the character")
	}
}

func TestTradeNeedsTheCharacterNearTheNPC(t *testing.T) {
	s := newStack(t)
	token := s.player("dave1")
	body := map[string]any{"offer_id": "x", "quantity": 1, "config_version": 1, "request_id": "trade-req-1"}
	code, out, _ := s.post("/api/v1/npc/FARM_SHOP/buy", token, body)
	if code != http.StatusConflict || out["error"].(map[string]any)["code"] != "NOT_IN_WORLD" {
		t.Fatalf("not in world: %d %v", code, out)
	}
	s.connect(token) // spawns at the plaza, far from the farm shop
	code, out, _ = s.post("/api/v1/npc/FARM_SHOP/buy", token, body)
	if code != http.StatusConflict || out["error"].(map[string]any)["code"] != "NPC_TOO_FAR" {
		t.Fatalf("far: %d %v", code, out)
	}
	select {
	case p := <-s.txHits:
		t.Fatalf("transaction service called: %s", p)
	default:
	}
}

func TestCSRFAndAuthGuards(t *testing.T) {
	s := newStack(t)
	token := s.player("erin1")
	raw, _ := json.Marshal(map[string]any{"request_id": "harvest-all-1"})
	req, _ := http.NewRequest(http.MethodPost, s.srv.URL+"/api/v1/farm/harvest-all", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Cookie", "thanh_session="+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-site write allowed: %d", resp.StatusCode)
	}
	if code, _, _ := s.get("/api/v1/state", "not-a-session"); code != http.StatusUnauthorized {
		t.Fatalf("bad cookie: %d", code)
	}
	// A legitimate write is forwarded with the server-side character id.
	if code, _, _ := s.post("/api/v1/farm/harvest-all", token, map[string]any{"request_id": "harvest-all-1"}); code != 200 {
		t.Fatalf("forward: %d", code)
	}
	if p := <-s.txHits; !strings.HasSuffix(p, "/commands/farm.harvest_all") {
		t.Fatalf("forwarded to %s", p)
	}
}
