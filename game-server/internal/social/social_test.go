package social_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/project-thanh/game-server/internal/social"
	"example.com/project-thanh/game-server/internal/testenv"
)

type env struct {
	t    *testing.T
	pool *pgxpool.Pool
	svc  *social.Service
	now  time.Time
	n    int
}

func newEnv(t *testing.T) *env {
	pool := testenv.DB(t)
	e := &env{t: t, pool: pool, svc: social.New(pool), now: time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC)}
	e.svc.Now = func() time.Time { return e.now }
	return e
}

// char creates an account + character; the search key mirrors names.SearchKey.
func (e *env) char(name, searchKey string) string {
	e.t.Helper()
	e.n++
	var acc, id string
	ctx := context.Background()
	if err := e.pool.QueryRow(ctx, `INSERT INTO accounts (username, username_key, password_hash) VALUES ($1, $1, 'x') RETURNING id::text`,
		fmt.Sprintf("u%d", e.n)).Scan(&acc); err != nil {
		e.t.Fatal(err)
	}
	if err := e.pool.QueryRow(ctx, `INSERT INTO characters (account_id, display_name, display_name_key, search_key) VALUES ($1, $2, $3, $4) RETURNING id::text`,
		acc, name, strings.ToLower(name), searchKey).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *env) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.pool.Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatal(err)
	}
}

func TestMutualRequestsBecomeOneFriendship(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a, b := e.char("An", "an"), e.char("Bình", "binh")
	r1, err := e.svc.SendRequest(ctx, a, b)
	if err != nil || r1.State != "pending" {
		t.Fatalf("%+v %v", r1, err)
	}
	again, _ := e.svc.SendRequest(ctx, a, b)
	if again.RequestID != r1.RequestID {
		t.Fatal("second request from the same sender created a duplicate")
	}
	r2, err := e.svc.SendRequest(ctx, b, a) // B asks back: accept instead of duplicate
	if err != nil || r2.State != "accepted" || r2.RequestID != r1.RequestID {
		t.Fatalf("%+v %v", r2, err)
	}
	if ok, _ := e.svc.AreFriends(ctx, a, b); !ok {
		t.Fatal("not friends")
	}
	if _, err := e.svc.SendRequest(ctx, a, b); !errors.Is(err, social.ErrAlreadyFriends) {
		t.Fatalf("got %v", err)
	}
	if _, err := e.svc.Decide(ctx, b, r1.RequestID, "accept"); !errors.Is(err, social.ErrNotPending) {
		t.Fatalf("decided twice: %v", err)
	}
	if _, err := e.svc.SendRequest(ctx, a, a); !errors.Is(err, social.ErrSelf) {
		t.Fatalf("self request: %v", err)
	}
}

func TestOnlyTheRightSideDecides(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a, b, c := e.char("An", "an"), e.char("Bé", "be"), e.char("Cò", "co")
	r, _ := e.svc.SendRequest(ctx, a, b)
	if _, err := e.svc.Decide(ctx, a, r.RequestID, "accept"); !errors.Is(err, social.ErrNotFound) {
		t.Fatalf("sender accepted own request: %v", err)
	}
	if _, err := e.svc.Decide(ctx, c, r.RequestID, "reject"); !errors.Is(err, social.ErrNotFound) {
		t.Fatalf("stranger rejected: %v", err)
	}
	if _, err := e.svc.Decide(ctx, a, r.RequestID, "cancel"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	in, out, _ := e.svc.Requests(ctx, b)
	if len(in) != 0 || len(out) != 0 {
		t.Fatalf("pending after cancel: %v %v", in, out)
	}
}

func TestS02FriendLimitHoldsEvenWithConcurrentAccepts(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	full := e.char("Đông Vui", "dong vui")
	for i := 0; i < 99; i++ {
		f := e.char(fmt.Sprintf("Bạn %d", i), fmt.Sprintf("ban %d", i))
		x, y := full, f
		if y < x {
			x, y = y, x
		}
		e.exec(`INSERT INTO friendships (character_a, character_b) VALUES ($1, $2)`, x, y)
	}
	// Two pending requests to the 99-friend character; both accepted at once.
	p1, p2 := e.char("Mới Một", "moi mot"), e.char("Mới Hai", "moi hai")
	r1, _ := e.svc.SendRequest(ctx, p1, full)
	r2, _ := e.svc.SendRequest(ctx, p2, full)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, r := range []string{r1.RequestID, r2.RequestID} {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			_, errs[i] = e.svc.Decide(ctx, full, id, "accept")
		}(i, r)
	}
	wg.Wait()
	ok, limited := 0, 0
	for _, err := range errs {
		var le *social.FriendLimitError
		switch {
		case err == nil:
			ok++
		case errors.As(err, &le) && le.Mine:
			limited++
		default:
			t.Fatalf("unexpected %v", err)
		}
	}
	if ok != 1 || limited != 1 {
		t.Fatalf("ok=%d limited=%d", ok, limited)
	}
	list, _ := e.svc.Friends(ctx, full)
	if len(list) != social.MaxFriends {
		t.Fatalf("%d friends", len(list))
	}
	// 101st by a new request: refused already when sending.
	extra := e.char("Thứ 101", "thu 101")
	if _, err := e.svc.SendRequest(ctx, full, extra); !errors.Is(err, social.ErrFriendLimit) {
		t.Fatalf("got %v", err)
	}
}

func TestS03S04HistoryPagesOf50Then20WithoutGapsOrRepeats(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a, b := e.char("An", "an"), e.char("Bình", "binh")
	for i := 0; i < 75; i++ {
		e.now = e.now.Add(time.Second)
		from, to := a, b
		if i%3 == 0 {
			from, to = b, a
		}
		if _, _, err := e.svc.SendPM(ctx, from, to, fmt.Sprintf("req-%06d", i), fmt.Sprintf("tin %d", i), 500); err != nil {
			t.Fatal(err)
		}
	}
	p1, err := e.svc.History(ctx, a, b, "")
	if err != nil || len(p1.Messages) != 50 || p1.NextCursor == nil {
		t.Fatalf("page 1: %d %v", len(p1.Messages), err)
	}
	if p1.Messages[49].Body != "tin 74" || p1.Messages[0].Body != "tin 25" {
		t.Fatalf("page 1 order: %s .. %s", p1.Messages[0].Body, p1.Messages[49].Body)
	}
	p2, _ := e.svc.History(ctx, b, a, *p1.NextCursor) // same conversation from the other side
	if len(p2.Messages) != 20 || p2.Messages[19].Body != "tin 24" || p2.Messages[0].Body != "tin 5" {
		t.Fatalf("page 2: %d %s..%s", len(p2.Messages), p2.Messages[0].Body, p2.Messages[19].Body)
	}
	p3, _ := e.svc.History(ctx, a, b, *p2.NextCursor)
	if len(p3.Messages) != 5 || p3.NextCursor != nil || p3.Messages[0].Body != "tin 0" {
		t.Fatalf("page 3: %+v", p3)
	}
	if _, err := e.svc.History(ctx, a, b, "garbage!"); !errors.Is(err, social.ErrBadCursor) {
		t.Fatalf("bad cursor: %v", err)
	}
}

func TestS05OnlyTheRecipientMarksVisibleMessagesSeen(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a, b := e.char("An", "an"), e.char("Bình", "binh")
	m1, _, _ := e.svc.SendPM(ctx, a, b, "req-aaaaaa1", "một", 500)
	e.now = e.now.Add(time.Second)
	m2, _, _ := e.svc.SendPM(ctx, a, b, "req-aaaaaa2", "hai", 500)
	if u, _ := e.svc.MarkSeen(ctx, a, []string{m1.ID}); len(u) != 0 {
		t.Fatal("the sender marked its own message seen")
	}
	u, err := e.svc.MarkSeen(ctx, b, []string{m1.ID})
	if err != nil || len(u) != 1 || u[0].SenderID != a || len(u[0].MessageIDs) != 1 {
		t.Fatalf("%+v %v", u, err)
	}
	page, _ := e.svc.History(ctx, b, a, "")
	if page.Messages[0].SeenAt == nil || page.Messages[1].SeenAt != nil {
		t.Fatal("seen state wrong: only the visible message may be seen")
	}
	if u, _ := e.svc.MarkSeen(ctx, b, []string{m1.ID}); len(u) != 0 {
		t.Fatal("seen twice")
	}
	conv, _ := e.svc.Conversations(ctx, b)
	if len(conv) != 1 || conv[0].Unread != 1 || conv[0].Last.ID != m2.ID {
		t.Fatalf("conversations %+v", conv)
	}
}

func TestS06S07ExpiryAndOneWaySilentBlock(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a, b := e.char("An", "an"), e.char("Bình", "binh")
	old, _, _ := e.svc.SendPM(ctx, b, a, "req-old0001", "cũ", 500)
	if !old.ExpiresAt.Equal(old.SentAt.Add(30 * 24 * time.Hour)) {
		t.Fatal("expiry is not sent_at + 30 days")
	}
	e.now = e.now.Add(30*24*time.Hour + time.Second)
	if page, _ := e.svc.History(ctx, a, b, ""); len(page.Messages) != 0 {
		t.Fatal("S-06: expired message returned")
	}
	// A blocks B: B's message fails with the generic error; A can still write.
	if err := e.svc.Block(ctx, a, b); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.svc.SendPM(ctx, b, a, "req-blocked1", "alo", 500); !errors.Is(err, social.ErrCannotDeliver) {
		t.Fatalf("S-07: %v", err)
	}
	if _, _, err := e.svc.SendPM(ctx, a, b, "req-fromA01", "chào", 500); err != nil {
		t.Fatalf("one-way block blocked the blocker: %v", err)
	}
	prof, _ := e.svc.Profile(ctx, b, a)
	if prof.BlockedByMe {
		t.Fatal("block leaked into the blocked person's profile")
	}
	// Unblock: nothing comes back, nothing is sent, no friendship.
	if err := e.svc.Unblock(ctx, a, b); err != nil {
		t.Fatal(err)
	}
	page, _ := e.svc.History(ctx, a, b, "")
	if len(page.Messages) != 1 || page.Messages[0].Body != "chào" {
		t.Fatalf("after unblock: %+v", page.Messages)
	}
	if ok, _ := e.svc.AreFriends(ctx, a, b); ok {
		t.Fatal("unblock befriended")
	}
	if _, err := e.svc.PurgeExpired(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestS10RetrySameRequestStoresOneMessage(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a, b := e.char("An", "an"), e.char("Bình", "binh")
	m1, ins1, err := e.svc.SendPM(ctx, a, b, "req-retry01", "xin chào", 500)
	m2, ins2, err2 := e.svc.SendPM(ctx, a, b, "req-retry01", "xin chào", 500)
	if err != nil || err2 != nil || !ins1 || ins2 || m1.ID != m2.ID {
		t.Fatalf("%v %v %v %v", ins1, ins2, err, err2)
	}
	if _, _, err := e.svc.SendPM(ctx, a, b, "req-long001", strings.Repeat("x", 501), 500); !errors.Is(err, social.ErrMessageTooLarge) {
		t.Fatalf("long message: %v", err)
	}
}

func TestS08S09BlockListPagesAndSearch(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	me := e.char("Tôi", "toi")
	var ids []string
	for i := 0; i < 55; i++ {
		e.now = e.now.Add(time.Second)
		name := fmt.Sprintf("Người %02d", i)
		key := fmt.Sprintf("nguoi %02d", i)
		if i == 7 {
			name, key = "Thành Đạt", "thanh dat"
		}
		id := e.char(name, key)
		ids = append(ids, id)
		if err := e.svc.Block(ctx, me, id); err != nil {
			t.Fatal(err)
		}
	}
	p1, _ := e.svc.Blocks(ctx, me, "", "")
	if len(p1.Items) != 50 || p1.NextCursor == nil || p1.Items[0].ID != ids[54] {
		t.Fatalf("page 1: %d, first %s", len(p1.Items), p1.Items[0].DisplayName)
	}
	p2, _ := e.svc.Blocks(ctx, me, "", *p1.NextCursor)
	if len(p2.Items) != 5 || p2.NextCursor != nil || p2.Items[4].ID != ids[0] {
		t.Fatalf("page 2: %+v", p2)
	}
	for _, q := range []string{"thanh", "THÀNH", "Đạt", "dat"} {
		res, _ := e.svc.Blocks(ctx, me, q, "")
		if len(res.Items) != 1 || res.Items[0].ID != ids[7] {
			t.Fatalf("search %q: %+v", q, res.Items)
		}
	}
	if res, _ := e.svc.Blocks(ctx, me, ids[3], ""); len(res.Items) != 1 || res.Items[0].ID != ids[3] {
		t.Fatal("full id search")
	}
	if res, _ := e.svc.Blocks(ctx, me, ids[3][:8], ""); len(res.Items) != 0 {
		t.Fatal("partial id must not match")
	}
	found, _ := e.svc.Search(ctx, me, "thành")
	if len(found) != 1 || found[0].ID != ids[7] {
		t.Fatalf("player search: %+v", found)
	}
	if found, _ := e.svc.Search(ctx, me, "%"); len(found) != 0 {
		t.Fatal("LIKE wildcard not escaped")
	}
}
