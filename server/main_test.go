package main

import (
	"encoding/json"
	"game/shared"
	"game/shared/world"
	"net"
	"testing"
	"time"
)

// newTestServer runs on the blank fallback grid, spawning at
// shared.SpawnPoint, as the server does without level files.
func newTestServer() *Server {
	return NewServer(&world.Map{Collider: world.NewGrid(world.FallbackSize, world.FallbackSize), Spawn: shared.SpawnPoint})
}

func sendMove(t *testing.T, s *Server, id uint64, addr string) {
	t.Helper()
	sendMoveTo(t, s, id, addr, shared.SpawnPoint)
}

func sendMoveTo(t *testing.T, s *Server, id uint64, addr string, pos shared.Vec2) {
	t.Helper()
	payload, _ := json.Marshal(shared.ClientMoveMsg{PlayerID: id, Position: pos, TimeMs: 50})
	data, _ := json.Marshal(shared.MessageWrapper{Type: "Move", Payload: payload})
	udp, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	s.handlePacket(data, udp)
}

func TestIdlePlayersAreDropped(t *testing.T) {
	s := newTestServer()
	sendMove(t, s, 1, "127.0.0.1:5001")
	sendMove(t, s, 2, "127.0.0.1:5002")
	s.lastSeen[1] = time.Now().Add(-playerTimeout - time.Second) // 1 went quiet

	s.dropIdle(time.Now())
	if _, ok := s.players[1]; ok {
		t.Error("idle player 1 is still in the broadcast")
	}
	if _, ok := s.suspicions[1]; ok {
		t.Error("idle player 1's suspicion tracker is still kept")
	}
	if _, ok := s.clients["127.0.0.1:5001"]; ok {
		t.Error("idle player 1's address is still sent broadcasts")
	}
	if _, ok := s.players[2]; !ok {
		t.Error("active player 2 was dropped")
	}

	// Coming back is just another first Move.
	sendMove(t, s, 1, "127.0.0.1:5001")
	if _, ok := s.players[1]; !ok {
		t.Error("player 1 couldn't rejoin")
	}
}

// A new player's first Move used to be validated against (0,0) while real
// clients start at the spawn point, so every connect was flagged too fast.
func TestFirstMoveFromSpawnIsNotFlagged(t *testing.T) {
	s := newTestServer()
	step := shared.Vec2{X: shared.SpawnPoint.X + 4, Y: shared.SpawnPoint.Y} // one tick's walk
	sendMoveTo(t, s, 1, "127.0.0.1:5001", step)

	if got := s.players[1].Position; got != step {
		t.Errorf("position = %v, want %v", got, step)
	}
	if tr := s.suspicions[1]; tr.Score != 0 || len(tr.Events) != 0 {
		t.Errorf("first move from spawn was flagged: score %v, events %v", tr.Score, tr.Events)
	}
}

// The spawn is the server's, not the client's: a first Move far from it is
// validated like any other jump, not accepted as a chosen start point.
func TestFirstMoveFarFromSpawnIsFlagged(t *testing.T) {
	s := newTestServer()
	sendMoveTo(t, s, 1, "127.0.0.1:5001", shared.Vec2{X: 1000, Y: 1000})

	tr := s.suspicions[1]
	if len(tr.Events) != 1 || tr.Events[0].Type != shared.SuspicionEventTooFast {
		t.Errorf("events = %v, want one TooFast", tr.Events)
	}
}

// The server spawns players at the world's spawn object, not the constant.
func TestPlayersSpawnAtTheWorldsSpawn(t *testing.T) {
	spawn := shared.Vec2{X: 40, Y: 56}
	s := NewServer(&world.Map{Collider: world.NewGrid(10, 10), Spawn: spawn})
	step := shared.Vec2{X: spawn.X + 4, Y: spawn.Y}
	sendMoveTo(t, s, 1, "127.0.0.1:5001", step)
	if tr := s.suspicions[1]; len(tr.Events) != 0 {
		t.Errorf("first move from the world's spawn was flagged: %v", tr.Events)
	}
}

// The repo's own sample world loads, and a short walk from its spawn is
// clean: what build_local.ps1 actually runs.
func TestSampleWorldWalkFromSpawnIsClean(t *testing.T) {
	m, err := world.LoadMap("..")
	if err != nil {
		t.Fatal(err)
	}
	if m.World == nil {
		t.Fatal("the repo root should have a sample world")
	}
	s := NewServer(m)
	pos := m.Spawn
	for i := 0; i < 20; i++ {
		pos.X += 4 // one network tick of walking
		sendMoveTo(t, s, 1, "127.0.0.1:5001", pos)
	}
	if tr := s.suspicions[1]; len(tr.Events) != 0 {
		t.Errorf("walking from the sample spawn was flagged: %v", tr.Events)
	}
}
