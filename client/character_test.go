package main

import (
	"game/client/anim"
	"game/shared"
	"testing"
)

func babyAnimator(t *testing.T) *CharacterAnimator {
	t.Helper()
	c, err := anim.NewLibrary().LoadCharacter("../assets/characters/baby/baby.anichar")
	if err != nil {
		t.Fatal(err)
	}
	return NewCharacterAnimator(&characterLook{lookDef: playerLook, char: c})
}

func sprite(t *testing.T, ca *CharacterAnimator) anim.Sprite {
	t.Helper()
	s := ca.AppendSprites(nil)
	if len(s) != 1 {
		t.Fatalf("%d sprites, want 1", len(s))
	}
	return s[0]
}

func TestJumpRunsToItsEndThenFollowsMovement(t *testing.T) {
	ca := babyAnimator(t)
	ca.Step(false, true, 4, 16)
	if ca.State() != shared.AnimJump {
		t.Fatalf("space didn't jump: state %d", ca.State())
	}
	seq := ca.Seq()
	// Moving or pressing jump again mid-jump doesn't cut it short or
	// restart it.
	for elapsed := uint32(0); elapsed < 790; elapsed += 10 {
		ca.Step(true, true, 4, 10)
		if ca.State() != shared.AnimJump || ca.Seq() != seq {
			t.Fatalf("at %dms: state %d seq %d, want the same jump", elapsed, ca.State(), ca.Seq())
		}
	}
	ca.Step(true, false, 4, 20) // reaches 800ms
	ca.Step(true, false, 4, 16)
	if ca.State() != shared.AnimWalk {
		t.Errorf("after the jump, moving: state %d, want walk", ca.State())
	}
	ca.Step(false, false, 4, 16)
	if ca.State() != shared.AnimIdle {
		t.Errorf("standing: state %d, want idle", ca.State())
	}
}

func TestBackToBackJumpsBumpSeq(t *testing.T) {
	ca := babyAnimator(t)
	ca.Step(false, true, 4, 16)
	first := ca.Seq()
	ca.Step(false, false, 4, 1000) // lands
	ca.Step(false, true, 4, 16)    // jumps again the very frame it lands
	if ca.State() != shared.AnimJump || ca.Seq() == first {
		t.Errorf("second jump: state %d seq %d (first was %d), want a new seq", ca.State(), ca.Seq(), first)
	}
	if s := sprite(t, ca); s.Y != 0 {
		t.Errorf("second jump didn't start from the top: y=%v", s.Y)
	}
}

func TestIdleHoldsWalksFirstFrame(t *testing.T) {
	ca := babyAnimator(t)
	ca.Apply(shared.AnimWalk, 0, 4, 300) // mid-stride
	ca.Apply(shared.AnimIdle, 0, 4, 16)
	if s := sprite(t, ca); s.Row != 0 || s.Col != 2 {
		t.Errorf("idle facing S drew %+v, want walk's first frame (0,2)", s)
	}
}

func TestRemoteJumpRestartsOnSeq(t *testing.T) {
	ca := babyAnimator(t)
	ca.Apply(shared.AnimWalk, 0, 2, 100)
	ca.Apply(shared.AnimJump, 1, 2, 16) // server now says jump: starts from the top
	if s := sprite(t, ca); s.Y != 0 || s.Row != 1 || s.Col != 1 {
		t.Errorf("jump's first frame facing E = %+v", s)
	}
	ca.Apply(shared.AnimJump, 1, 2, 396)
	if s := sprite(t, ca); s.Y != -69 {
		t.Errorf("jump apex y = %v, want -69", s.Y)
	}
	// Still jumping, new seq: the player jumped again before the state
	// ever left jump.
	ca.Apply(shared.AnimJump, 2, 2, 16)
	if s := sprite(t, ca); s.Y != 0 {
		t.Errorf("new seq didn't restart the jump: y=%v", s.Y)
	}
}

func TestUnknownStateShowsIdle(t *testing.T) {
	ca := babyAnimator(t)
	ca.Apply(200, 0, 4, 16) // from a newer client
	if ca.State() != shared.AnimIdle {
		t.Errorf("unknown state became %d, want idle", ca.State())
	}
}

func TestPlayersMissingFromABroadcastAreDropped(t *testing.T) {
	c := &Client{
		playerID:      1,
		controller:    NewPlayerController(shared.Vec2{}, shared.NewCollisionLayer(10, 10)),
		remotePlayers: map[uint64]*PlayerInterpolation{},
	}
	c.applyPlayerStates([]shared.PlayerState{{PlayerID: 1}, {PlayerID: 2}, {PlayerID: 3}})
	if len(c.remotePlayers) != 2 {
		t.Fatalf("%d remote players, want 2 and 3", len(c.remotePlayers))
	}
	c.applyPlayerStates([]shared.PlayerState{{PlayerID: 1}, {PlayerID: 3}})
	if _, ok := c.remotePlayers[2]; ok {
		t.Error("player 2 left the broadcast but is still drawn")
	}
	if _, ok := c.remotePlayers[3]; !ok {
		t.Error("player 3 was dropped")
	}
}
