package main

import (
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"

	goaway "github.com/TwiN/go-away"
	tea "github.com/charmbracelet/bubbletea"
)

// ChatRoom is an in-memory, ephemeral broadcast channel. Membership is tied
// to the user actively viewing the chat page (Join on enter, Leave on exit
// or disconnect). Messages are only delivered to current members and are
// kept in a small ring buffer so newcomers see a bit of recent context.
// Nothing is persisted across server restarts.

const (
	chatBacklogSize    = 30
	chatMaxMessageLen  = 200
	chatMaxClientLines = 100

	// chatRateLimitMax messages allowed per chatRateLimitWindow per sender.
	// Excess sends are dropped server-side and a private system notice is
	// returned to the offender only.
	chatRateLimitMax    = 5
	chatRateLimitWindow = 10 * time.Second
)

// chatSender is the minimum surface ChatRoom needs to deliver a message to a
// subscriber. *tea.Program satisfies it; tests use stubs.
type chatSender interface {
	Send(tea.Msg)
}

type chatSub struct {
	program  chatSender
	username string
}

type ChatRoom struct {
	mu          sync.Mutex
	subscribers map[string]*chatSub
	recent      []message
	rate        map[string][]time.Time
	now         func() time.Time
}

func NewChatRoom() *ChatRoom {
	return &ChatRoom{
		subscribers: make(map[string]*chatSub),
		rate:        make(map[string][]time.Time),
		now:         time.Now,
	}
}

// Join registers the fingerprint as a current viewer of the chat page. It
// returns the recent backlog so the joiner can render some context. If this
// is a fresh join, a system message is broadcast to the rest of the room.
// Idempotent: rejoining (e.g. after closing the menu) does not re-broadcast.
func (c *ChatRoom) Join(fp string, p chatSender, username string) []message {
	c.mu.Lock()
	backlog := append([]message(nil), c.recent...)
	if !chatSenderReady(p) {
		c.mu.Unlock()
		return backlog
	}
	_, already := c.subscribers[fp]
	c.subscribers[fp] = &chatSub{program: p, username: username}
	subs := c.snapshotLocked()
	count := len(subs)
	now := c.now()
	c.mu.Unlock()

	if !already && username != "" {
		fanout(subs, fp, message{system: true, sender: username, content: "joined", at: now})
	}
	broadcastPresence(subs, count)
	return backlog
}

// Leave removes the fingerprint from the room, notifies others, and drops
// its rate-limit state. Safe to call when the user was never a member.
func (c *ChatRoom) Leave(fp string) {
	c.mu.Lock()
	prev, was := c.subscribers[fp]
	delete(c.subscribers, fp)
	delete(c.rate, fp)
	subs := c.snapshotLocked()
	count := len(subs)
	now := c.now()
	c.mu.Unlock()

	if !was {
		return
	}
	if prev.username != "" {
		fanout(subs, "", message{system: true, sender: prev.username, content: "left", at: now})
	}
	broadcastPresence(subs, count)
}

// Broadcast fan-out; empty dropped. Rate limit or abuse → private system line (abuse checked first).
func (c *ChatRoom) Broadcast(senderFP string, msg message) tea.Cmd {
	msg.content = strings.TrimSpace(msg.content)
	if msg.content == "" {
		return nil
	}
	if len(msg.content) > chatMaxMessageLen {
		msg.content = msg.content[:chatMaxMessageLen]
	}
	msg.system = false

	if notice := chatAbuseNotice(msg.content); notice != "" {
		now := c.now()
		return func() tea.Msg {
			return message{system: true, content: notice, at: now}
		}
	}

	c.mu.Lock()
	now := c.now()
	if !c.allowLocked(senderFP, now) {
		c.mu.Unlock()
		notice := message{
			system:  true,
			content: "you're sending messages too fast — slow down",
			at:      now,
		}
		return func() tea.Msg { return notice }
	}
	msg.at = now
	c.recent = append(c.recent, msg)
	if len(c.recent) > chatBacklogSize {
		c.recent = c.recent[len(c.recent)-chatBacklogSize:]
	}
	subs := c.snapshotLocked()
	c.mu.Unlock()

	fanout(subs, senderFP, msg)
	return func() tea.Msg { return msg }
}

// allowLocked checks and updates the per-sender sliding window. Caller holds
// c.mu. Returns true if the message is allowed (and records its timestamp).
func (c *ChatRoom) allowLocked(fp string, now time.Time) bool {
	cutoff := now.Add(-chatRateLimitWindow)
	hist := c.rate[fp]
	pruned := hist[:0]
	for _, t := range hist {
		if t.After(cutoff) {
			pruned = append(pruned, t)
		}
	}
	if len(pruned) >= chatRateLimitMax {
		c.rate[fp] = pruned
		return false
	}
	c.rate[fp] = append(pruned, now)
	return true
}

// IsPrint (+ tab/NL/ZWNJ/ZWJ); go-away profanity.
func chatAbuseNotice(content string) string {
	if chatHasDisallowedRunes(content) {
		return "that message was blocked (invalid characters)"
	}
	if goaway.IsProfane(content) {
		return "that message wasn't appropriate for this chat"
	}
	return ""
}

func chatHasDisallowedRunes(s string) bool {
	for _, r := range s {
		if !chatRuneAllowed(r) {
			return true
		}
	}
	return false
}

func chatRuneAllowed(r rune) bool {
	switch r {
	case '\t', '\n', '\r', '\u200c', '\u200d':
		return true
	default:
		return unicode.IsPrint(r)
	}
}

func (c *ChatRoom) snapshotLocked() map[string]*chatSub {
	subs := make(map[string]*chatSub, len(c.subscribers))
	for k, v := range c.subscribers {
		subs[k] = v
	}
	return subs
}

// fanout delivers msg to every subscriber whose fingerprint != skipFP via
// the subscriber's chatSender. Sends are launched as goroutines so a slow
// or closing program never blocks the broadcaster.
func fanout(subs map[string]*chatSub, skipFP string, msg tea.Msg) {
	for fp, sub := range subs {
		if fp == skipFP || sub == nil || !chatSenderReady(sub.program) {
			continue
		}
		prog := sub.program
		go prog.Send(msg)
	}
}

func broadcastPresence(subs map[string]*chatSub, count int) {
	fanout(subs, "", presenceMsg{count: count})
}

func chatSenderReady(sender chatSender) bool {
	if sender == nil {
		return false
	}

	// A typed-nil pointer stored in an interface compares non-nil, so check
	// the dynamic value before attempting to send.
	v := reflect.ValueOf(sender)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !v.IsNil()
	default:
		return true
	}
}
