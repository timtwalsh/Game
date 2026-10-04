package anim

import "testing"

// BenchmarkFrame is one character's per-frame animation cost.
func BenchmarkFrame(b *testing.B) {
	c, err := NewLibrary().LoadCharacter(babyPath)
	if err != nil {
		b.Fatal(err)
	}
	inst := NewInstance(c)
	inst.Play("walk")
	var buf []Sprite
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		inst.SetFacing(uint8(i % 8))
		inst.Advance(7)
		_ = inst.Finished()
		buf = inst.AppendSprites(buf[:0])
	}
}
