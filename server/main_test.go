package main

import (
	"encoding/json"
	"game/shared"
	"net"
	"testing"
	"time"
)

func sendMove(t *testing.T, s *Server, id uint64, addr string) {
	t.Helper()
	payload, _ := json.Marshal(shared.ClientMoveMsg{PlayerID: id, TimeMs: 50})
	data, _ := json.Marshal(shared.MessageWrapper{Type: "Move", Payload: payload})
	udp, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	s.handlePacket(data, udp)
}

func TestIdlePlayersAreDropped(t *testing.T) {
	s := NewServer()
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
