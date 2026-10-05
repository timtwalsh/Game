package world

// Compile rebuilds the level's compiled property grids (D30):
//
//   - blocking: the ground terrain's flags OR every placed tile's flags
//     (strictest wins); empty ground blocks everything.
//   - surface, interaction: the topmost tile that declares one, else the
//     ground terrain's, else id 0.
//   - an artist override on a map beats everything, and leaves the other
//     two maps alone.
//
// The ground layer's own tile counts as a placed tile, so a hand-picked tile
// on a locked ground cell (D3) can declare properties too.
func Compile(l *Level, defs *Defs) {
	for i := range l.Ground.Terrain {
		var blocking uint16
		var surface, interaction uint8
		haveSurface, haveInteraction := false, false

		take := func(tile uint16) {
			if tile == 0 {
				return
			}
			p := defs.TileProps(tile)
			blocking |= p.Blocks
			if p.HasSurface && !haveSurface {
				surface, haveSurface = p.Surface, true
			}
			if p.HasInteraction && !haveInteraction {
				interaction, haveInteraction = p.Interaction, true
			}
		}
		for j := len(l.Upper) - 1; j >= 0; j-- {
			take(l.Upper[j].Tile[i])
		}
		take(l.Ground.Tile[i])

		if t := defs.Terrain(l.Ground.Terrain[i]); t != nil {
			blocking |= t.Blocks
			if !haveSurface {
				surface = t.SurfaceID
			}
			if !haveInteraction {
				interaction = t.InteractionID
			}
		} else {
			// Empty ground, or a terrain these defs don't know: fail closed.
			blocking = BlockAll
		}

		o := l.Overrides.Mask[i]
		if o&OverrideBlocking != 0 {
			blocking = l.Overrides.Blocking[i]
		}
		if o&OverrideSurface != 0 {
			surface = l.Overrides.Surface[i]
		}
		if o&OverrideInteraction != 0 {
			interaction = l.Overrides.Interaction[i]
		}
		l.Props.Blocking[i] = blocking
		l.Props.Surface[i] = surface
		l.Props.Interaction[i] = interaction
	}
}

// SetOverride sets one property map's override at level-local (x, y).
// which is one of OverrideBlocking, OverrideSurface or OverrideInteraction;
// value is the flag mask or id. It does not recompile.
func (l *Level) SetOverride(x, y int, which uint8, value uint16) {
	if !l.In(x, y) {
		return
	}
	i := l.Index(x, y)
	l.Overrides.Mask[i] |= which
	switch which {
	case OverrideBlocking:
		l.Overrides.Blocking[i] = value
	case OverrideSurface:
		l.Overrides.Surface[i] = uint8(value)
	case OverrideInteraction:
		l.Overrides.Interaction[i] = uint8(value)
	}
}

// ClearOverride removes one property map's override at level-local (x, y).
func (l *Level) ClearOverride(x, y int, which uint8) {
	if !l.In(x, y) {
		return
	}
	i := l.Index(x, y)
	l.Overrides.Mask[i] &^= which
	switch which {
	case OverrideBlocking:
		l.Overrides.Blocking[i] = 0
	case OverrideSurface:
		l.Overrides.Surface[i] = 0
	case OverrideInteraction:
		l.Overrides.Interaction[i] = 0
	}
}
