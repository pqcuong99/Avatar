package api

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"example.com/avatar/game-server/internal/httpx"
	"example.com/avatar/game-server/internal/social"
	"example.com/avatar/game-server/internal/world"
)

var (
	errAlreadyFriends = &httpx.Error{Status: http.StatusConflict, Code: "ALREADY_FRIENDS", Message: "Hai bạn đã là bạn bè."}
	errFriendLimit    = &httpx.Error{Status: http.StatusConflict, Code: "FRIEND_LIMIT_REACHED", Message: "Danh sách bạn bè đã đủ 100 người."}
	errNotPending     = &httpx.Error{Status: http.StatusConflict, Code: "REQUEST_NOT_PENDING", Message: "Lời mời này đã được xử lý."}
	errNotFriends     = &httpx.Error{Status: http.StatusConflict, Code: "NOT_FRIENDS", Message: "Hai bạn chưa là bạn bè."}
	errRoomFull       = &httpx.Error{Status: http.StatusConflict, Code: "ROOM_FULL", Message: "Phòng của bạn ấy đã đầy."}
	errFriendOffline  = &httpx.Error{Status: http.StatusConflict, Code: "FRIEND_OFFLINE", Message: "Bạn ấy không ở trên bản đồ lúc này."}
	errSameRoom       = &httpx.Error{Status: http.StatusConflict, Code: "SAME_ROOM", Message: "Bạn đã ở cùng phòng."}
	// Deliberately says nothing about blocks (TH-06-002); FORBIDDEN per docs/07 §7.
	errCannotDeliver = &httpx.Error{Status: http.StatusForbidden, Code: "FORBIDDEN", Message: "Không gửi được tin nhắn."}
	errMessageLength = &httpx.Error{Status: http.StatusUnprocessableEntity, Code: "VALIDATION_FAILED", Message: "Tin nhắn trống hoặc quá dài."}
)

func (a *API) socialRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/players/search", a.withCharacter(a.searchPlayers))
	mux.HandleFunc("GET /api/v1/players/{characterID}/profile", a.withCharacter(a.profile))
	mux.HandleFunc("GET /api/v1/friends", a.withCharacter(a.friends))
	mux.HandleFunc("GET /api/v1/friends/requests", a.withCharacter(a.friendRequests))
	mux.HandleFunc("POST /api/v1/friends/requests", a.withCharacter(a.sendFriendRequest))
	mux.HandleFunc("POST /api/v1/friends/requests/{requestID}/{action}", a.withCharacter(a.decideFriendRequest))
	mux.HandleFunc("DELETE /api/v1/friends/{characterID}", a.withCharacter(a.removeFriend))
	mux.HandleFunc("POST /api/v1/friends/{characterID}/join-room", a.withCharacter(a.joinFriendRoom))
	mux.HandleFunc("GET /api/v1/private-chat/conversations", a.withCharacter(a.conversations))
	mux.HandleFunc("GET /api/v1/private-chat/blocks", a.withCharacter(a.blocks))
	mux.HandleFunc("POST /api/v1/private-chat/blocks/{characterID}", a.withCharacter(a.block))
	mux.HandleFunc("DELETE /api/v1/private-chat/blocks/{characterID}", a.withCharacter(a.unblock))
	mux.HandleFunc("POST /api/v1/private-chat/seen", a.withCharacter(a.seen))
	mux.HandleFunc("POST /api/v1/private-chat/messages/{messageID}/seen", a.withCharacter(a.seenOne))
	// "/with/" avoids overlapping "/blocks/{characterID}" (docs/07 draft used
	// /private-chat/{characterId}/messages).
	mux.HandleFunc("GET /api/v1/private-chat/with/{characterID}/messages", a.withCharacter(a.history))
	mux.HandleFunc("POST /api/v1/private-chat/with/{characterID}/messages", a.withCharacter(a.sendPM))
}

func (a *API) socialFail(w http.ResponseWriter, err error) {
	var limit *social.FriendLimitError
	switch {
	case errors.As(err, &limit):
		httpx.Fail(w, errFriendLimit.With(map[string]any{"mine": limit.Mine}))
	case errors.Is(err, social.ErrNotFound), errors.Is(err, social.ErrBadCursor):
		httpx.Fail(w, httpx.NotFound)
	case errors.Is(err, social.ErrSelf):
		httpx.Fail(w, httpx.Validation.With(map[string]any{"reason": "self"}))
	case errors.Is(err, social.ErrAlreadyFriends):
		httpx.Fail(w, errAlreadyFriends)
	case errors.Is(err, social.ErrNotPending):
		httpx.Fail(w, errNotPending)
	case errors.Is(err, social.ErrNotFriends):
		httpx.Fail(w, errNotFriends)
	case errors.Is(err, social.ErrCannotDeliver):
		httpx.Fail(w, errCannotDeliver)
	case errors.Is(err, social.ErrMessageTooLarge):
		httpx.Fail(w, errMessageLength)
	case errors.Is(err, world.ErrRoomFull):
		httpx.Fail(w, errRoomFull)
	case errors.Is(err, world.ErrFriendOffline):
		httpx.Fail(w, errFriendOffline)
	case errors.Is(err, world.ErrSameRoom):
		httpx.Fail(w, errSameRoom)
	case errors.Is(err, world.ErrNotJoined):
		httpx.Fail(w, httpx.NotInWorld)
	default:
		a.Log.Error("social", "err", err)
		httpx.Fail(w, httpx.Internal)
	}
}

func (a *API) searchPlayers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if len(q) > 64 {
		httpx.Fail(w, httpx.Validation)
		return
	}
	res, err := a.Social.Search(r.Context(), character(r).ID, q)
	if err != nil {
		a.socialFail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"players": res})
}

func (a *API) profile(w http.ResponseWriter, r *http.Request) {
	me := character(r).ID
	p, err := a.Social.Profile(r.Context(), me, r.PathValue("characterID"))
	if err != nil {
		a.socialFail(w, err)
		return
	}
	out := map[string]any{"profile": p}
	// Presence is shown to friends only. [ĐỀ XUẤT] — TH-07-004 leaves it open.
	if p.IsFriend {
		pr := a.World.PresenceOf(r.Context(), []string{p.ID, me})
		out["presence"] = pr[p.ID]
		out["my_room_id"] = pr[me].RoomID
	}
	httpx.OK(w, out)
}

type friendView struct {
	social.Friend
	Presence world.Presence `json:"presence"`
}

func (a *API) friends(w http.ResponseWriter, r *http.Request) {
	me := character(r).ID
	list, err := a.Social.Friends(r.Context(), me)
	if err != nil {
		a.socialFail(w, err)
		return
	}
	ids := []string{me}
	for _, f := range list {
		ids = append(ids, f.ID)
	}
	pr := a.World.PresenceOf(r.Context(), ids)
	out := make([]friendView, 0, len(list))
	for _, f := range list {
		out = append(out, friendView{Friend: f, Presence: pr[f.ID]})
	}
	httpx.OK(w, map[string]any{"friends": out, "max": social.MaxFriends, "my_room_id": pr[me].RoomID})
}

func (a *API) friendRequests(w http.ResponseWriter, r *http.Request) {
	in, outgoing, err := a.Social.Requests(r.Context(), character(r).ID)
	if err != nil {
		a.socialFail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"incoming": in, "outgoing": outgoing})
}

func (a *API) sendFriendRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CharacterID string `json:"character_id"`
	}
	if err := decode(r, &body); err != nil {
		httpx.Fail(w, httpx.Validation)
		return
	}
	me := character(r)
	res, err := a.Social.SendRequest(r.Context(), me.ID, body.CharacterID)
	if err != nil {
		a.socialFail(w, err)
		return
	}
	from := social.Player{ID: me.ID, DisplayName: me.DisplayName}
	if res.State == "accepted" {
		a.World.SendTo(body.CharacterID, world.Message("friend.accepted", "", map[string]any{"friend": from}))
	} else {
		a.World.SendTo(body.CharacterID, world.Message("friend.request", "", map[string]any{"request_id": res.RequestID, "from": from}))
	}
	httpx.OK(w, res)
}

func (a *API) decideFriendRequest(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("action")
	if action != "accept" && action != "reject" && action != "cancel" {
		httpx.Fail(w, httpx.NotFound)
		return
	}
	me := character(r)
	other, err := a.Social.Decide(r.Context(), me.ID, r.PathValue("requestID"), action)
	if err != nil {
		a.socialFail(w, err)
		return
	}
	if action == "accept" {
		a.World.SendTo(other.ID, world.Message("friend.accepted", "", map[string]any{"friend": social.Player{ID: me.ID, DisplayName: me.DisplayName}}))
	}
	httpx.OK(w, map[string]any{"other": other, "action": action})
}

func (a *API) removeFriend(w http.ResponseWriter, r *http.Request) {
	if err := a.Social.RemoveFriend(r.Context(), character(r).ID, r.PathValue("characterID")); err != nil {
		a.socialFail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"removed": true})
}

func (a *API) joinFriendRoom(w http.ResponseWriter, r *http.Request) {
	me, target := character(r).ID, r.PathValue("characterID")
	ok, err := a.Social.AreFriends(r.Context(), me, target)
	if err != nil {
		a.socialFail(w, err)
		return
	}
	if !ok {
		a.socialFail(w, social.ErrNotFriends)
		return
	}
	room, err := a.World.JoinFriendRoom(r.Context(), me, target)
	if err != nil {
		a.socialFail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"room_id": room})
}

// ---------------------------------------------------------------------------
// Private chat
// ---------------------------------------------------------------------------

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

// pmLimiter spaces out one sender's private messages. [ĐỀ XUẤT] value; the
// chat limits are [CẦN CẤU HÌNH] (GAP-11).
type pmLimiter struct {
	mu   sync.Mutex
	last map[string]time.Time
}

var pmLimit = &pmLimiter{last: map[string]time.Time{}}

func (l *pmLimiter) allow(sender string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if now.Sub(l.last[sender]) < 300*time.Millisecond {
		return false
	}
	l.last[sender] = now
	if len(l.last) > 10000 {
		for k, t := range l.last {
			if now.Sub(t) > time.Minute {
				delete(l.last, k)
			}
		}
	}
	return true
}

const privateMaxChars = 500 // technical bound while GAP-11 is open

func cleanText(s string) string {
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

func (a *API) sendPM(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Body      string `json:"body"`
		RequestID string `json:"request_id"`
	}
	if err := decode(r, &body); err != nil {
		httpx.Fail(w, httpx.Validation)
		return
	}
	reqID := body.RequestID
	if k := r.Header.Get("Idempotency-Key"); k != "" {
		reqID = k
	}
	if !requestIDPattern.MatchString(reqID) {
		httpx.Fail(w, httpx.Validation.With(map[string]any{"field": "request_id"}))
		return
	}
	me := character(r)
	if !pmLimit.allow(me.ID) {
		httpx.Fail(w, httpx.RateLimited)
		return
	}
	target := r.PathValue("characterID")
	msg, inserted, err := a.Social.SendPM(r.Context(), me.ID, target, reqID, cleanText(body.Body), privateMaxChars)
	if err != nil {
		a.socialFail(w, err)
		return
	}
	if inserted {
		a.World.SendTo(target, world.Message("chat.private.message", "", map[string]any{
			"message": msg, "sender": social.Player{ID: me.ID, DisplayName: me.DisplayName}}))
	}
	httpx.OK(w, map[string]any{"message": msg, "replayed": !inserted})
}

func (a *API) history(w http.ResponseWriter, r *http.Request) {
	page, err := a.Social.History(r.Context(), character(r).ID, r.PathValue("characterID"), r.URL.Query().Get("before"))
	if err != nil {
		a.socialFail(w, err)
		return
	}
	httpx.OK(w, page)
}

func (a *API) conversations(w http.ResponseWriter, r *http.Request) {
	list, err := a.Social.Conversations(r.Context(), character(r).ID)
	if err != nil {
		a.socialFail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"conversations": list})
}

func (a *API) markSeen(ctx context.Context, w http.ResponseWriter, me string, ids []string) {
	updates, err := a.Social.MarkSeen(ctx, me, ids)
	if err != nil {
		a.socialFail(w, err)
		return
	}
	for _, u := range updates {
		a.World.SendTo(u.SenderID, world.Message("chat.private.seen", "", map[string]any{
			"reader_id": me, "message_ids": u.MessageIDs, "seen_at": u.SeenAt}))
	}
	n := 0
	for _, u := range updates {
		n += len(u.MessageIDs)
	}
	httpx.OK(w, map[string]any{"marked": n})
}

// seen marks messages the client reports as actually visible in its
// viewport (TH-05-003).
func (a *API) seen(w http.ResponseWriter, r *http.Request) {
	var body struct {
		MessageIDs []string `json:"message_ids"`
	}
	if err := decode(r, &body); err != nil || len(body.MessageIDs) > 200 {
		httpx.Fail(w, httpx.Validation)
		return
	}
	a.markSeen(r.Context(), w, character(r).ID, body.MessageIDs)
}

func (a *API) seenOne(w http.ResponseWriter, r *http.Request) {
	a.markSeen(r.Context(), w, character(r).ID, []string{r.PathValue("messageID")})
}

func (a *API) blocks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if len(q.Get("q")) > 64 {
		httpx.Fail(w, httpx.Validation)
		return
	}
	page, err := a.Social.Blocks(r.Context(), character(r).ID, q.Get("q"), q.Get("cursor"))
	if err != nil {
		a.socialFail(w, err)
		return
	}
	httpx.OK(w, page)
}

func (a *API) block(w http.ResponseWriter, r *http.Request) {
	if err := a.Social.Block(r.Context(), character(r).ID, r.PathValue("characterID")); err != nil {
		a.socialFail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"blocked": true})
}

func (a *API) unblock(w http.ResponseWriter, r *http.Request) {
	if err := a.Social.Unblock(r.Context(), character(r).ID, r.PathValue("characterID")); err != nil {
		a.socialFail(w, err)
		return
	}
	httpx.OK(w, map[string]any{"blocked": false})
}
