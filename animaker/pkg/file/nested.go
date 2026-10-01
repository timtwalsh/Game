package file

import (
	"fmt"
	"os"

	"animaker/pkg/editor"
)

// LoadNestedAnim loads one .anif to play inside another, with its own
// sheets, and records it in loaded (keyed by editor.AnimKey). Any nested
// animations it references in turn are loaded too; one already in loaded
// isn't loaded again, which is also what stops an animation that contains
// itself from recursing forever. problems describes anything that couldn't
// be loaded along the way (a missing .anif or sheet), for the artist.
func LoadNestedAnim(path string, loaded map[string]*editor.NestedAnim) (anim *editor.NestedAnim, problems []string) {
	key := editor.AnimKey(path)
	if a, ok := loaded[key]; ok {
		return a, nil
	}
	if _, err := os.Stat(key); err != nil {
		return nil, []string{fmt.Sprintf("nested animation %s: not found", key)}
	}
	track, refs, err := LoadTrack(key)
	if err != nil {
		return nil, []string{fmt.Sprintf("nested animation %s: %v", key, err)}
	}
	sheets, missing, sheetProblems := LoadSheetsForTrack(key, refs, track.ReferencedSheetNames())
	problems = append(problems, sheetProblems...)
	for _, name := range missing {
		problems = append(problems, fmt.Sprintf("nested animation %s: sheet %q not found", key, name))
	}

	anim = &editor.NestedAnim{Path: key, Track: track, Sheets: sheets}
	// Recorded before recursing, so a cycle finds it and stops.
	loaded[key] = anim
	problems = append(problems, LoadNestedAnimsFor(track, loaded)...)
	return anim, problems
}

// LoadNestedAnimsFor loads every nested animation a track references
// (nested parts and animation props' defaults) into loaded.
func LoadNestedAnimsFor(track *editor.Track, loaded map[string]*editor.NestedAnim) (problems []string) {
	for _, p := range track.ReferencedAnimPaths() {
		_, probs := LoadNestedAnim(p, loaded)
		problems = append(problems, probs...)
	}
	return problems
}
