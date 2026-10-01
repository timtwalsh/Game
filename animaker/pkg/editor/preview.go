package editor

import (
	"fmt"
	"image"
	"path/filepath"
	"sort"
	"strings"
)

// LoadPreviewSheet slices img as a stand-in for a prop's art and makes it
// that prop's preview override, returning the name it was registered
// under. It is for trying a recolour or alternate sheet ("actually show
// this as sprite_red.png") without touching the track.
//
// The image is sliced exactly as the prop's default sheet is — same cell
// size, same pivot — because that is what a swap means at runtime: a
// replacement sheet supplies new pixels for the same template grid, and
// (Row, Col) keeps meaning the same pose (see docs/ANI_MAKER_SPEC.md,
// "Sprite sheet templates"). Asking for a grid here would let the preview
// show something the game never could.
//
// A preview sheet lives only in PreviewSheets: it is never saved (no
// .sprsh is written, and PreviewProps isn't part of the .anif), and it is
// not offered as a part's fixed sheet, a prop default or a palette sheet,
// so nothing authored can come to depend on it.
func (p *Project) LoadPreviewSheet(propName, filePath string, img image.Image) (string, error) {
	prop := p.CurrentTrack.FindProp(propName)
	if prop == nil {
		return "", fmt.Errorf("no prop called %q", propName)
	}
	base := p.LoadedSheets[prop.Default]
	if base == nil {
		return "", fmt.Errorf("prop %q's default sheet %q isn't loaded, so there's no grid to slice by", propName, prop.Default)
	}

	name := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	// A preview must not shadow an imported sheet of the same name, since
	// sheet lookups try LoadedSheets first.
	if _, taken := p.LoadedSheets[name]; taken || name == "" {
		name += " (preview)"
	}

	if p.PreviewSheets == nil {
		p.PreviewSheets = map[string]*SpriteSheetTemplate{}
	}
	p.PreviewSheets[name] = NewSpriteSheetTemplate(name, filePath, img, base.CellW, base.CellH, base.PivotX, base.PivotY)
	p.PreviewProps[propName] = name
	return name, nil
}

// PreviewOptionNames lists what a prop's preview override can be set to:
// every imported sheet, then every preview-only sheet, each sorted.
func (p *Project) PreviewOptionNames() []string {
	names := p.LoadedSheetNames()
	previews := make([]string, 0, len(p.PreviewSheets))
	for n := range p.PreviewSheets {
		previews = append(previews, n)
	}
	sort.Strings(previews)
	return append(names, previews...)
}

// lookupSheet finds a sheet by name among imported sheets, then preview
// sheets.
func (p *Project) lookupSheet(name string) *SpriteSheetTemplate {
	if s := p.LoadedSheets[name]; s != nil {
		return s
	}
	return p.PreviewSheets[name]
}
