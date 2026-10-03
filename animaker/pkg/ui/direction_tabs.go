package ui

import (
	"animaker/pkg/editor"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// DirectionTabs is the direction bar's row of facings: one button per
// direction of the track, named by compass point (N, NE, E...) rather than
// as bare ints, each with its keyframe count so
// unposed facings stand out without visiting them (they're also drawn in
// the warning colour). The active one is highlighted.
//
// Plain buttons rather than a widget.Select, which re-fires its change
// handler on every programmatic SetSelected - the trap noted on
// PropertiesPanel.refreshPartList.
type DirectionTabs struct {
	box *fyne.Container

	// MenuFor builds the menu for a direction (right-click on its tab, or
	// the Directions button for the active one).
	MenuFor func(key int) *fyne.Menu

	// OnSelect fires when the artist clicks a direction other than the
	// active one.
	OnSelect func(key int)
}

// directionTabColumns is how many tabs sit on a row: the four standard
// facings fit the right column side by side; more wrap.
const directionTabColumns = 4

func NewDirectionTabs() *DirectionTabs {
	return &DirectionTabs{box: container.NewVBox()}
}

// Object is the widget to place in a layout.
func (dt *DirectionTabs) Object() fyne.CanvasObject { return dt.box }

// Refresh rebuilds the tabs from the track's directions.
func (dt *DirectionTabs) Refresh(track *editor.Track, active int) {
	keys := track.SortedDirectionKeys()
	facings := track.Facings()
	cols := len(keys)
	if cols > directionTabColumns {
		cols = directionTabColumns
	}
	if cols == 0 {
		cols = 1
	}
	grid := container.NewGridWithColumns(cols)
	for _, k := range keys {
		key := k
		n := track.Directions[key].TotalKeyframes()
		btn := newDirTab(DirectionTabLabel(key, facings, n), func() {
			if key != active && dt.OnSelect != nil {
				dt.OnSelect(key)
			}
		}, func(e *fyne.PointEvent) { dt.showMenu(key, e.AbsolutePosition) })
		switch {
		case key == active:
			btn.Importance = widget.HighImportance
		case n == 0:
			btn.Importance = widget.WarningImportance
		}
		grid.Add(btn)
	}
	dt.box.Objects = []fyne.CanvasObject{grid}
	dt.box.Refresh()
}

// showMenu pops up the direction menu for key (MenuFor) at an absolute
// position - a right-click on its tab, or the Directions button.
func (dt *DirectionTabs) showMenu(key int, at fyne.Position) {
	if dt.MenuFor == nil {
		return
	}
	if c := fyne.CurrentApp().Driver().CanvasForObject(dt.box); c != nil {
		widget.ShowPopUpMenuAtPosition(dt.MenuFor(key), c, at)
	}
}

// ShowMenuBelow pops up key's menu under obj - for the Directions button,
// which acts on the active direction.
func (dt *DirectionTabs) ShowMenuBelow(key int, obj fyne.CanvasObject) {
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(obj)
	dt.showMenu(key, pos.AddXY(0, obj.Size().Height))
}

// dirTab is a direction tab: a button that also opens the direction's
// menu on right-click (delete, change key...).
type dirTab struct {
	widget.Button
	onSecondary func(*fyne.PointEvent)
}

var _ fyne.SecondaryTappable = (*dirTab)(nil)

func newDirTab(label string, onTap func(), onSecondary func(*fyne.PointEvent)) *dirTab {
	b := &dirTab{onSecondary: onSecondary}
	b.Text, b.OnTapped = label, onTap
	b.ExtendBaseWidget(b)
	return b
}

func (b *dirTab) TappedSecondary(e *fyne.PointEvent) { b.onSecondary(e) }

// DirectionTabLabel is a tab's text: the facing's compass name (key k of
// an n-direction track) and how many keyframes it has, so "(0)" marks a
// facing not posed yet.
func DirectionTabLabel(k, n, keyframes int) string {
	return fmt.Sprintf("%s (%d)", editor.FacingName(k, n), keyframes)
}
