package ui

import (
	"animaker/pkg/editor"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// DirectionTabs is the direction bar's row of facings: one button per
// direction of the track, named the way the game numbers them ("Up",
// "Right"...) rather than as bare ints, each with its keyframe count so
// unposed facings stand out without visiting them (they're also drawn in
// the warning colour). The active one is highlighted.
//
// Plain buttons rather than a widget.Select, which re-fires its change
// handler on every programmatic SetSelected - the trap noted on
// PropertiesPanel.refreshPartList.
type DirectionTabs struct {
	box *fyne.Container

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
		btn := widget.NewButton(DirectionTabLabel(key, n), func() {
			if key != active && dt.OnSelect != nil {
				dt.OnSelect(key)
			}
		})
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

// directionName is a facing's name in the game's convention, or "Dir N"
// for any other key (diagonals, extra facings).
func directionName(k int) string {
	switch k {
	case 0:
		return "Up"
	case 1:
		return "Right"
	case 2:
		return "Down"
	case 3:
		return "Left"
	}
	return fmt.Sprintf("Dir %d", k)
}

// DirectionTabLabel is a tab's text: the facing's name and how many
// keyframes it has, so "(0)" marks a facing not posed yet.
func DirectionTabLabel(k, keyframes int) string {
	return fmt.Sprintf("%s (%d)", directionName(k), keyframes)
}
