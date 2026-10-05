package main

import (
	"fmt"
	"game/shared"
	"io"
	"strings"
	"sync"
	"time"
)

// StatusBoard is the server terminal's live view: one block per player
// showing whether they're connected and the latest thing they did,
// redrawn in place rather than scrolling a log. Players are listed in
// the order they first connected.
type StatusBoard struct {
	mu       sync.Mutex
	header   string
	order    []uint64
	entries  map[uint64]*boardEntry
	nextNum  int
	events   []string // recent server-level messages (errors etc.), newest last
	dirty    bool
	now      func() time.Time
	inPlace  bool // terminal understands ANSI cursor movement
	lastRows int
}

type boardEntry struct {
	num            int
	addr           string
	connected      bool
	action         string
	actionAt       time.Time
	disconnectedAt time.Time
}

const (
	// boardMaxEvents is how many recent server-level messages stay on screen.
	boardMaxEvents = 5
	// boardForgetAfter is how long a disconnected player stays listed.
	boardForgetAfter = time.Minute
	// boardRefresh is how often the board redraws (only when something changed).
	boardRefresh = 100 * time.Millisecond
)

func NewStatusBoard() *StatusBoard {
	return &StatusBoard{
		entries: make(map[uint64]*boardEntry),
		nextNum: 1,
		now:     time.Now,
		dirty:   true,
	}
}

func (b *StatusBoard) SetHeader(h string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.header = h
	b.dirty = true
}

func (b *StatusBoard) Connected(id uint64, addr string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	e, ok := b.entries[id]
	if !ok {
		e = &boardEntry{num: b.nextNum}
		b.nextNum++
		b.entries[id] = e
		b.order = append(b.order, id)
	}
	e.addr = addr
	e.connected = true
	e.action = "connected"
	e.actionAt = b.now()
	b.dirty = true
}

func (b *StatusBoard) Disconnected(id uint64, reason string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	e, ok := b.entries[id]
	if !ok {
		return
	}
	e.connected = false
	e.action = reason
	e.actionAt = b.now()
	e.disconnectedAt = e.actionAt
	b.dirty = true
}

// Action records the latest thing a player did, replacing the previous one.
func (b *StatusBoard) Action(id uint64, action string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	e, ok := b.entries[id]
	if !ok || e.action == action {
		return
	}
	e.action = action
	e.actionAt = b.now()
	b.dirty = true
}

// Event records a server-level message that isn't about one player.
func (b *StatusBoard) Event(format string, args ...any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	msg := b.now().Format("15:04:05") + "  " + fmt.Sprintf(format, args...)
	b.events = append(b.events, msg)
	if len(b.events) > boardMaxEvents {
		b.events = b.events[len(b.events)-boardMaxEvents:]
	}
	b.dirty = true
}

// Render returns the board as text, dropping players who have been
// disconnected longer than boardForgetAfter.
func (b *StatusBoard) Render() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.render()
}

func (b *StatusBoard) render() string {
	now := b.now()
	kept := b.order[:0]
	for _, id := range b.order {
		e := b.entries[id]
		if !e.connected && now.Sub(e.disconnectedAt) > boardForgetAfter {
			delete(b.entries, id)
			continue
		}
		kept = append(kept, id)
	}
	b.order = kept

	online := 0
	for _, e := range b.entries {
		if e.connected {
			online++
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s  (%d online)\n\n", b.header, online)
	if len(b.order) == 0 {
		sb.WriteString("No players yet\n")
	}
	for _, id := range b.order {
		e := b.entries[id]
		status := "connected"
		if !e.connected {
			status = "disconnected"
		}
		fmt.Fprintf(&sb, "Player %d %s  [%s, id %d]\n", e.num, status, e.addr, id)
		fmt.Fprintf(&sb, "> %-40s %s\n", e.action, e.actionAt.Format("15:04:05"))
	}
	if len(b.events) > 0 {
		sb.WriteString("\nRecent:\n")
		for _, ev := range b.events {
			sb.WriteString("  " + ev + "\n")
		}
	}
	return sb.String()
}

// Run redraws the board to w whenever it changes, until done is closed.
func (b *StatusBoard) Run(w io.Writer, done <-chan struct{}) {
	b.inPlace = enableANSI()
	ticker := time.NewTicker(boardRefresh)
	defer ticker.Stop()
	for {
		b.draw(w)
		select {
		case <-ticker.C:
		case <-done:
			return
		}
	}
}

func (b *StatusBoard) draw(w io.Writer) {
	b.mu.Lock()
	// Disconnected players age off the board, so it must also re-render
	// when one expires, even if nothing new happened.
	if !b.dirty && !b.hasExpired(b.now()) {
		b.mu.Unlock()
		return
	}
	b.dirty = false
	text := b.render()
	b.mu.Unlock()

	if !b.inPlace {
		// No cursor control: print each frame in full, separated.
		fmt.Fprint(w, text+strings.Repeat("-", 40)+"\n")
		return
	}
	// Home the cursor, overwrite each line (clearing its tail), then clear
	// anything left below from a longer previous frame. Avoids the flicker
	// of a full-screen clear.
	var sb strings.Builder
	sb.WriteString("\x1b[H")
	for _, line := range strings.SplitAfter(text, "\n") {
		sb.WriteString(strings.TrimSuffix(line, "\n"))
		if strings.HasSuffix(line, "\n") {
			sb.WriteString("\x1b[K\n")
		}
	}
	sb.WriteString("\x1b[J")
	fmt.Fprint(w, sb.String())
}

func (b *StatusBoard) hasExpired(now time.Time) bool {
	for _, e := range b.entries {
		if !e.connected && now.Sub(e.disconnectedAt) > boardForgetAfter {
			return true
		}
	}
	return false
}

var directionNames = [...]string{"N", "NE", "E", "SE", "S", "SW", "W", "NW"}

func directionName(d uint8) string {
	if int(d) < len(directionNames) {
		return directionNames[d]
	}
	return "?"
}

// describeMove names what changed between a player's previous state and
// the one their latest Move reports, or "" if nothing worth showing did.
// Position alone isn't an action - walking is, and is described by the
// animation state and facing the client reports.
func describeMove(prev shared.PlayerState, msg shared.ClientMoveMsg) string {
	jumpStarted := msg.Animation == shared.AnimJump &&
		(prev.Animation != shared.AnimJump || prev.AnimSeq != msg.AnimSeq)
	switch {
	case jumpStarted:
		return "jumped facing " + directionName(msg.Direction)
	case msg.Animation == shared.AnimWalk &&
		(prev.Animation != shared.AnimWalk || prev.Direction != msg.Direction):
		return "walking " + directionName(msg.Direction)
	case msg.Animation == shared.AnimIdle && prev.Animation != shared.AnimIdle:
		return fmt.Sprintf("stopped at (%.0f, %.0f) facing %s", msg.Position.X, msg.Position.Y, directionName(msg.Direction))
	}
	return ""
}
