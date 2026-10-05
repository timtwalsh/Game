package main

import (
	"game/shared"
	"strings"
	"testing"
	"time"
)

func newTestBoard() (*StatusBoard, *time.Time) {
	clock := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	b := NewStatusBoard()
	b.now = func() time.Time { return clock }
	b.SetHeader("Server listening on test")
	return b, &clock
}

func TestStatusBoardShowsLatestActionPerPlayerInConnectOrder(t *testing.T) {
	b, _ := newTestBoard()
	b.Connected(100, "a:1")
	b.Connected(200, "b:2")
	b.Action(100, "walking E")
	b.Action(100, "jumped facing E") // replaces, doesn't append
	b.Disconnected(200, "timed out")

	got := b.Render()
	p1 := strings.Index(got, "Player 1 connected")
	p2 := strings.Index(got, "Player 2 disconnected")
	if p1 < 0 || p2 < 0 || p1 > p2 {
		t.Fatalf("want Player 1 connected listed before Player 2 disconnected, got:\n%s", got)
	}
	if strings.Contains(got, "walking E") || !strings.Contains(got, "> jumped facing E") {
		t.Errorf("want only the latest action for Player 1, got:\n%s", got)
	}
	if !strings.Contains(got, "> timed out") {
		t.Errorf("want Player 2's disconnect reason as its latest action, got:\n%s", got)
	}
	if !strings.Contains(got, "(1 online)") {
		t.Errorf("want online count of 1, got:\n%s", got)
	}
}

func TestStatusBoardForgetsLongDisconnectedPlayers(t *testing.T) {
	b, clock := newTestBoard()
	b.Connected(100, "a:1")
	b.Connected(200, "b:2")
	b.Disconnected(100, "timed out")

	*clock = clock.Add(boardForgetAfter + time.Second)
	got := b.Render()
	if strings.Contains(got, "id 100") {
		t.Errorf("player disconnected over %v ago should be gone, got:\n%s", boardForgetAfter, got)
	}
	if !strings.Contains(got, "Player 2 connected") {
		t.Errorf("connected player should keep its number, got:\n%s", got)
	}
}

func TestStatusBoardKeepsOnlyRecentEvents(t *testing.T) {
	b, _ := newTestBoard()
	for i := 0; i < boardMaxEvents+2; i++ {
		b.Event("event %d", i)
	}
	got := b.Render()
	if strings.Contains(got, "event 0") || !strings.Contains(got, "event 6") {
		t.Errorf("want only the last %d events, got:\n%s", boardMaxEvents, got)
	}
}

func TestDescribeMove(t *testing.T) {
	idleN := shared.PlayerState{Animation: shared.AnimIdle, Direction: 0}
	walkE := shared.PlayerState{Animation: shared.AnimWalk, Direction: 2}
	cases := []struct {
		name string
		prev shared.PlayerState
		msg  shared.ClientMoveMsg
		want string
	}{
		{"start walking", idleN, shared.ClientMoveMsg{Animation: shared.AnimWalk, Direction: 2}, "walking E"},
		{"keep walking same way", walkE, shared.ClientMoveMsg{Animation: shared.AnimWalk, Direction: 2}, ""},
		{"turn while walking", walkE, shared.ClientMoveMsg{Animation: shared.AnimWalk, Direction: 3}, "walking SE"},
		{"stop", walkE, shared.ClientMoveMsg{Animation: shared.AnimIdle, Direction: 2, Position: shared.Vec2{X: 40, Y: 8}}, "stopped at (40, 8) facing E"},
		{"stay idle", idleN, shared.ClientMoveMsg{Animation: shared.AnimIdle}, ""},
		{"jump", idleN, shared.ClientMoveMsg{Animation: shared.AnimJump, AnimSeq: 1}, "jumped facing N"},
		{"back-to-back jump", shared.PlayerState{Animation: shared.AnimJump, AnimSeq: 1}, shared.ClientMoveMsg{Animation: shared.AnimJump, AnimSeq: 2}, "jumped facing N"},
		{"mid-jump", shared.PlayerState{Animation: shared.AnimJump, AnimSeq: 1}, shared.ClientMoveMsg{Animation: shared.AnimJump, AnimSeq: 1}, ""},
	}
	for _, c := range cases {
		if got := describeMove(c.prev, c.msg); got != c.want {
			t.Errorf("%s: describeMove = %q, want %q", c.name, got, c.want)
		}
	}
}
