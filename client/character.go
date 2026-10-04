package main

import (
	"game/client/anim"
	"game/shared"
)

// animState is how one of the shared.Anim* states plays. The state names
// what the player is doing; this table maps it to a .anichar animation.
// Adding a state (attack, hurt) is a shared.Anim* constant, a row here,
// and whatever starts it in nextLocalState.
type animState struct {
	anim string // the .anichar animation it plays
	// standIn is shown, held on its first frame, when the character has
	// no anim ("idle" without an idle animation stands on walk's first
	// frame). Without one, a missing animation falls back to idle.
	standIn string
	// oneShot states start from the top each time they're entered and run
	// to their end before the local player can leave them.
	oneShot bool
}

var animStates = [...]animState{
	shared.AnimIdle: {anim: "idle", standIn: "walk"},
	shared.AnimWalk: {anim: "walk"},
	shared.AnimJump: {anim: "jump", oneShot: true},
}

// CharacterAnimator is the engine's animation state machine for one
// player. The assets only name animations and their play modes;
// transitions live here, never in an .anif or .anichar.
//
// The local player's animator decides its state (Step); a remote
// player's is told it by the server (Apply). seq counts one-shot starts,
// so a remote client restarts a jump it's already showing when the
// player jumps again.
type CharacterAnimator struct {
	look  *characterLook // shared by every player drawn the same way
	inst  *anim.Instance
	state uint8
	seq   uint8
}

func NewCharacterAnimator(look *characterLook) *CharacterAnimator {
	ca := &CharacterAnimator{look: look, inst: anim.NewInstance(look.char)}
	ca.Apply(shared.AnimIdle, 0, 4, 0)
	return ca
}

// Step advances the local player's animation: a one-shot runs to its end
// before anything else; otherwise jumpPressed starts a jump, and walking
// or standing follows movement.
func (ca *CharacterAnimator) Step(moving, jumpPressed bool, facing uint8, deltaMs uint32) {
	state := ca.nextLocalState(moving, jumpPressed)
	seq := ca.seq
	if animStates[state].oneShot && (state != ca.state || ca.inst.Finished()) {
		seq++
	}
	ca.Apply(state, seq, facing, deltaMs)
}

func (ca *CharacterAnimator) nextLocalState(moving, jumpPressed bool) uint8 {
	if animStates[ca.state].oneShot && !ca.inst.Finished() {
		return ca.state
	}
	if jumpPressed && ca.inst.Has(animStates[shared.AnimJump].anim) {
		return shared.AnimJump
	}
	if moving {
		return shared.AnimWalk
	}
	return shared.AnimIdle
}

// Apply plays state facing the given 8-way direction and advances it by
// deltaMs. A one-shot starts from the top when the state or seq changes.
// A state this client doesn't know (from a newer client) shows as idle.
func (ca *CharacterAnimator) Apply(state, seq, facing uint8, deltaMs uint32) {
	inst := ca.inst
	if int(state) >= len(animStates) {
		state = shared.AnimIdle
	}
	st := animStates[state]
	if !inst.Has(st.anim) && st.standIn == "" {
		state, st = shared.AnimIdle, animStates[shared.AnimIdle]
	}
	entering := state != ca.state || seq != ca.seq
	ca.state, ca.seq = state, seq
	inst.SetFacing(facing)

	switch {
	case !inst.Has(st.anim):
		inst.Play(st.standIn)
		inst.Seek(0)
	case entering && st.oneShot:
		inst.Restart(st.anim) // its first frame shows now; time starts next frame
	default:
		inst.Play(st.anim)
		inst.Advance(deltaMs)
	}
}

// State and Seq are what the client sends for the local player.
func (ca *CharacterAnimator) State() uint8 { return ca.state }
func (ca *CharacterAnimator) Seq() uint8   { return ca.seq }

// AppendSprites appends this frame's sprites to dst; see
// anim.Instance.AppendSprites.
func (ca *CharacterAnimator) AppendSprites(dst []anim.Sprite) []anim.Sprite {
	return ca.inst.AppendSprites(dst)
}
