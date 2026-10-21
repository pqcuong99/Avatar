// Package realtime is the WebSocket endpoint (docs/08): the session cookie is
// checked before the upgrade, then JSON text frames carry low-frequency events
// and binary frames carry movement (P0-166).
package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/coder/websocket"

	"example.com/project-thanh/game-server/internal/authn"
	"example.com/project-thanh/game-server/internal/gamedb"
	"example.com/project-thanh/game-server/internal/httpx"
	"example.com/project-thanh/game-server/internal/proto"
	"example.com/project-thanh/game-server/internal/social"
	"example.com/project-thanh/game-server/internal/txclient"
	"example.com/project-thanh/game-server/internal/world"
)

type Handler struct {
	World          *world.World
	Auth           *authn.Store
	DB             *gamedb.Store
	Social         *social.Service
	Tx             *txclient.Client
	CookieName     string
	OriginPatterns []string
	Log            *slog.Logger
	// Revalidate is how often a live connection re-checks its session, so a
	// revoked or superseded session also ends on other server instances.
	Revalidate time.Duration
}

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(h.CookieName)
	if err != nil {
		httpx.Fail(w, httpx.AuthRequired)
		return
	}
	principal, err := h.Auth.Authenticate(r.Context(), cookie.Value)
	if err != nil {
		if errors.Is(err, authn.ErrBanned) {
			httpx.Fail(w, httpx.AccountBanned)
			return
		}
		httpx.Fail(w, httpx.AuthRequired)
		return
	}
	char, err := h.DB.CharacterByAccount(r.Context(), principal.AccountID)
	if errors.Is(err, gamedb.ErrNoCharacter) {
		httpx.Fail(w, httpx.CharacterRequired)
		return
	}
	if err != nil {
		h.Log.Error("load character", "err", err)
		httpx.Fail(w, httpx.Internal)
		return
	}
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.OriginPatterns})
	if err != nil {
		return // Accept already wrote the HTTP error
	}
	ws.SetReadLimit(8 << 10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn := newConn(ws, cancel)
	conn.characterID = char.ID
	go conn.writeLoop(ctx)
	go h.revalidate(ctx, conn, cookie.Value)

	look, err := h.DB.Appearance(ctx, char.ID)
	if err != nil {
		h.Log.Error("load appearance", "err", err)
	}
	id := world.Identity{CharacterID: char.ID, AccountID: char.AccountID, DisplayName: char.DisplayName,
		MapID: char.MapID, X: char.X, Y: char.Y, HasPosition: char.MapID != "", Appearance: look}
	if _, err := h.World.Join(ctx, conn, id); err != nil {
		code := "WORLD_UNAVAILABLE"
		if errors.Is(err, world.ErrNotConfigured) {
			code = "WORLD_NOT_CONFIGURED"
		}
		conn.SendText(world.Message("error", "", map[string]any{"code": code, "message": "Chưa thể vào bản đồ lúc này."}))
		conn.Close(world.CloseNotConfigured, strings.ToLower(code))
		<-ctx.Done()
		return
	}
	defer h.World.Disconnect(conn)
	h.readLoop(ctx, conn)
}

// revalidate ends the connection when its session stops being valid.
func (h *Handler) revalidate(ctx context.Context, conn *wsConn, token string) {
	every := h.Revalidate
	if every <= 0 {
		every = time.Minute
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c, cancel := context.WithTimeout(ctx, 5*time.Second)
			_, err := h.Auth.Authenticate(c, token)
			cancel()
			if errors.Is(err, authn.ErrNoSession) || errors.Is(err, authn.ErrBanned) {
				conn.Close(world.CloseSessionReplaced, "session_invalid")
				return
			}
		}
	}
}

// Message budget per connection: generous for 20 Hz movement plus chat.
const maxMessagesPerSecond = 60

func (h *Handler) readLoop(ctx context.Context, conn *wsConn) {
	windowStart, count := time.Now(), 0
	for {
		typ, data, err := conn.ws.Read(ctx)
		if err != nil {
			return
		}
		if now := time.Now(); now.Sub(windowStart) >= time.Second {
			windowStart, count = now, 0
		}
		count++
		if count > maxMessagesPerSecond {
			conn.Close(world.CloseProtocolError, "too_many_messages")
			return
		}
		if typ == websocket.MessageBinary {
			in, ok, err := proto.DecodeClientFrame(data)
			if err != nil {
				conn.Close(world.CloseProtocolError, "bad_frame")
				return
			}
			if ok {
				h.World.Input(conn, in)
			}
			continue
		}
		var env world.Envelope
		if err := json.Unmarshal(data, &env); err != nil || env.V != world.ProtocolVersion {
			conn.Close(world.CloseProtocolError, "bad_envelope")
			return
		}
		h.handleText(ctx, conn, env)
	}
}

func (h *Handler) handleText(ctx context.Context, conn *wsConn, env world.Envelope) {
	switch env.Type {
	case "ping":
		var p struct {
			ClientTime int64 `json:"client_time"`
		}
		_ = json.Unmarshal(env.Payload, &p)
		conn.SendText(world.Message("pong", env.RequestID, map[string]any{"client_time": p.ClientTime, "server_time": time.Now().UTC()}))
	case "client.ready":
		var p struct {
			MapVersion string `json:"map_version"`
		}
		_ = json.Unmarshal(env.Payload, &p)
		if ok, current := h.World.CheckMapVersion(ctx, conn, p.MapVersion); !ok {
			conn.SendText(world.Message("error", env.RequestID, map[string]any{
				"code": "MAP_VERSION_MISMATCH", "message": "Bản đồ đã được cập nhật, hãy tải lại trang.", "map_version": current}))
		}
	case "chat.public.send":
		h.publicChat(ctx, conn, env)
	case "chat.private.typing":
		h.typing(ctx, conn, env)
	default:
		conn.SendText(world.Message("error", env.RequestID, map[string]any{"code": "UNKNOWN_MESSAGE", "message": "Loại tin không hỗ trợ."}))
	}
}

func chatError(conn *wsConn, requestID, code, msg string) {
	conn.SendText(world.Message("chat.public.failed", requestID, map[string]any{"code": code, "message": msg}))
}

// publicChat validates, stores once per request_id and then delivers to the
// sender's room. A retried request_id is acknowledged to the sender only and
// never broadcast twice (S-10).
func (h *Handler) publicChat(ctx context.Context, conn *wsConn, env world.Envelope) {
	if !requestIDPattern.MatchString(env.RequestID) {
		chatError(conn, env.RequestID, "VALIDATION_FAILED", "Thiếu mã yêu cầu.")
		return
	}
	var p struct {
		Body string `json:"body"`
	}
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		chatError(conn, env.RequestID, "VALIDATION_FAILED", "Tin nhắn không hợp lệ.")
		return
	}
	body := cleanChat(p.Body)
	sender, maxChars, err := h.World.ChatPrecheck(ctx, conn)
	switch {
	case errors.Is(err, world.ErrRateLimited):
		chatError(conn, env.RequestID, "RATE_LIMITED", "Bạn gửi tin quá nhanh.")
		return
	case err != nil:
		chatError(conn, env.RequestID, "NOT_IN_WORLD", "Chưa vào phòng.")
		return
	}
	if n := utf8.RuneCountInString(body); n == 0 || n > maxChars {
		chatError(conn, env.RequestID, "VALIDATION_FAILED", "Tin nhắn trống hoặc quá dài.")
		return
	}
	dbctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	msg, inserted, err := h.DB.InsertPublicChat(dbctx, sender.CharacterID, sender.RoomID, env.RequestID, body)
	if err != nil {
		h.Log.Error("store public chat", "err", err)
		chatError(conn, env.RequestID, "INTERNAL", "Không gửi được tin, hãy thử lại.")
		return
	}
	out := world.Message("chat.public.delivered", env.RequestID, map[string]any{
		"message_id": msg.ID, "room_id": msg.RoomID, "body": msg.Body, "sent_at": msg.SentAt.UTC(), "sender": sender,
	})
	if !inserted {
		conn.SendText(out)
		return
	}
	h.World.BroadcastRoom(sender.RoomID, out)
	h.countChatQuest(sender.CharacterID, msg.ID)
}

// countChatQuest reports the message to the daily quest engine. The message
// id is the request id, so a retry is never counted twice; failures only
// cost quest progress and are logged.
func (h *Handler) countChatQuest(characterID, messageID string) {
	if h.Tx == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		status, body, err := h.Tx.Do(ctx, http.MethodPost, "/internal/v1/characters/"+characterID+"/commands/quest.event",
			map[string]any{"request_id": "chat-" + messageID, "args": map[string]any{"event_type": "public_chat", "amount": 1}})
		if err != nil || status != http.StatusOK {
			h.Log.Warn("chat quest progress not recorded", "status", status, "err", err, "body", string(body))
		}
	}()
}

// typing relays the private-chat typing indicator (TH-05-004). Nothing is
// relayed to someone who blocked the typist, and nothing tells the typist.
func (h *Handler) typing(ctx context.Context, conn *wsConn, env world.Envelope) {
	if h.Social == nil {
		return
	}
	now := time.Now()
	if now.Sub(conn.lastTyping) < 900*time.Millisecond {
		return
	}
	conn.lastTyping = now
	var p struct {
		To string `json:"to"`
	}
	if json.Unmarshal(env.Payload, &p) != nil || !social.IsUUID(p.To) || p.To == conn.characterID {
		return
	}
	blocked, err := h.Social.IsBlocked(ctx, p.To, conn.characterID)
	if err != nil || blocked {
		return
	}
	h.World.SendTo(p.To, world.Message("chat.private.typing", "", map[string]any{"from": conn.characterID}))
}

// cleanChat trims the message and drops control characters; rendering always
// treats it as plain text (no HTML), so nothing else needs escaping here.
func cleanChat(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

// wsConn implements world.Conn. Sends never block: a full buffer means the
// client cannot keep up and the connection is closed.
type wsConn struct {
	ws          *websocket.Conn
	out         chan frame
	done        chan struct{}
	closeOnce   sync.Once
	closeCode   int
	closeReason string
	cancel      context.CancelFunc
	// Read-loop only:
	characterID string
	lastTyping  time.Time
}

type frame struct {
	binary bool
	data   []byte
}

func newConn(ws *websocket.Conn, cancel context.CancelFunc) *wsConn {
	return &wsConn{ws: ws, out: make(chan frame, 256), done: make(chan struct{}), cancel: cancel}
}

func (c *wsConn) SendText(b []byte) bool   { return c.enqueue(frame{data: b}) }
func (c *wsConn) SendBinary(b []byte) bool { return c.enqueue(frame{binary: true, data: b}) }

func (c *wsConn) enqueue(f frame) bool {
	select {
	case <-c.done:
		return false
	default:
	}
	select {
	case c.out <- f:
		return true
	default:
		return false
	}
}

// Close asks the writer to flush what is queued (e.g. a final error message)
// and then close the socket with the given code.
func (c *wsConn) Close(code int, reason string) {
	c.closeOnce.Do(func() {
		c.closeCode, c.closeReason = code, reason
		close(c.done)
	})
}

func (c *wsConn) writeLoop(ctx context.Context) {
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	defer c.cancel()
	for {
		select {
		case <-ctx.Done():
			_ = c.ws.CloseNow()
			return
		case <-c.done:
			for {
				select {
				case f := <-c.out:
					if c.write(ctx, f) != nil {
						_ = c.ws.CloseNow()
						return
					}
					continue
				default:
				}
				break
			}
			_ = c.ws.Close(websocket.StatusCode(c.closeCode), c.closeReason)
			return
		case f := <-c.out:
			if err := c.write(ctx, f); err != nil {
				_ = c.ws.CloseNow()
				return
			}
		case <-ping.C:
			pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := c.ws.Ping(pctx)
			cancel()
			if err != nil {
				_ = c.ws.CloseNow()
				return
			}
		}
	}
}

func (c *wsConn) write(ctx context.Context, f frame) error {
	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	typ := websocket.MessageText
	if f.binary {
		typ = websocket.MessageBinary
	}
	return c.ws.Write(wctx, typ, f.data)
}
