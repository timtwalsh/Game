package ui

import (
	"image"
	"testing"

	"animaker/pkg/editor"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// Regression: reported as "weird ui overlapping" once the selected
// keyframe and preview overrides joined the parts pane. Selecting a part
// rebuilds the sections inside one shared column, and the column never
// re-laid out, so each section stayed where it was when it was empty and
// they drew on top of each other.
func TestRigSectionsDoNotOverlapAfterSelectingAPart(t *testing.T) {
	test.NewTempApp(t)

	p := editor.NewProject("test")
	p.LoadedSheets["sprite"] = editor.NewSpriteSheetTemplate("sprite", "sprite.png",
		image.NewRGBA(image.Rect(0, 0, 32, 32)), 16, 16, 8, 8)
	editor.EnsureProp(p.CurrentTrack, "body", "sprite")
	idx, kf := p.DropTile("sprite", 0, 0, 0, 0)

	pp := NewPropertiesPanel(p)
	w := test.NewWindow(pp.Build(widget.NewLabel("dir")))
	defer w.Close()
	w.Resize(fyne.NewSize(440, 1200))

	pp.SelectPart(idx)
	p.Selection.KeyframeIndex = kf.ID
	pp.Refresh()

	sections := []struct {
		name string
		box  *fyne.Container
	}{
		{"part list", pp.partListBox},
		{"part link", pp.partLinkBox},
		{"keyframe", pp.keyframeBox},
		{"preview", pp.previewBox},
	}
	for i := 1; i < len(sections); i++ {
		above, below := sections[i-1], sections[i]
		bottom := above.box.Position().Y + above.box.Size().Height
		if below.box.Position().Y < bottom {
			t.Errorf("%s starts at y=%v, inside %s which ends at y=%v",
				below.name, below.box.Position().Y, above.name, bottom)
		}
		if below.box.Size().Height < below.box.MinSize().Height {
			t.Errorf("%s is %vpx tall, less than the %vpx it needs",
				below.name, below.box.Size().Height, below.box.MinSize().Height)
		}
	}
}
