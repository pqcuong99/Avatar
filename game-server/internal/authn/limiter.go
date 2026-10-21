package authn

import (
	"sync"
	"time"
)

// Limiter is the in-process layer of the anti-brute-force policy (P0-145):
//
//   - per account name: after FreeFailures failed logins each further attempt
//     must wait an exponentially growing delay, capped at MaxDelay. It is
//     never a permanent lock, and a successful login clears it;
//   - per IP: at most PerIPPerMinute attempts per minute.
//
// One process only. Several Game Server replicas need the same counters in
// Redis, and CAPTCHA on suspicious traffic is not built yet (GAP-16).
type Limiter struct {
	FreeFailures   int
	MaxDelay       time.Duration
	PerIPPerMinute int
	Now            func() time.Time

	mu       sync.Mutex
	accounts map[string]*failures
	ips      map[string]*window
}

type failures struct {
	count int
	last  time.Time
}

type window struct {
	start time.Time
	count int
}

func NewLimiter() *Limiter {
	return &Limiter{FreeFailures: 5, MaxDelay: 5 * time.Minute, PerIPPerMinute: 30, Now: time.Now,
		accounts: map[string]*failures{}, ips: map[string]*window{}}
}

// Allow reports whether an attempt may proceed now, and otherwise how long
// to wait.
func (l *Limiter) Allow(ip, account string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.Now()
	if ip != "" {
		w := l.ips[ip]
		if w == nil || now.Sub(w.start) >= time.Minute {
			w = &window{start: now}
			l.ips[ip] = w
		}
		if w.count >= l.PerIPPerMinute {
			return false, time.Minute - now.Sub(w.start)
		}
		w.count++
	}
	if f := l.accounts[account]; f != nil && f.count >= l.FreeFailures {
		delay := time.Second << min(f.count-l.FreeFailures, 20)
		if delay > l.MaxDelay {
			delay = l.MaxDelay
		}
		if wait := f.last.Add(delay).Sub(now); wait > 0 {
			return false, wait
		}
	}
	return true, 0
}

func (l *Limiter) Failure(account string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f := l.accounts[account]
	if f == nil {
		f = &failures{}
		l.accounts[account] = f
	}
	f.count++
	f.last = l.Now()
}

func (l *Limiter) Success(account string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.accounts, account)
}

// Sweep drops stale entries; call it periodically.
func (l *Limiter) Sweep() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.Now()
	for k, f := range l.accounts {
		if now.Sub(f.last) > 24*time.Hour {
			delete(l.accounts, k)
		}
	}
	for k, w := range l.ips {
		if now.Sub(w.start) > time.Minute {
			delete(l.ips, k)
		}
	}
}
