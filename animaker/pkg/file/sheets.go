package file

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"animaker/pkg/editor"

	"github.com/BurntSushi/toml"
)

// Bounds on the fallback folder search, so opening a track saved somewhere
// huge (a Desktop, a drive root) stays quick.
const (
	sheetSearchMaxDepth   = 3
	sheetSearchMaxEntries = 5000
)

// LoadSheetsForTrack loads the sprite sheets a just-opened track needs.
// It tries the paths the .anif recorded (refs) first, then, for any needed
// name still unresolved, searches the .anif's folder and its subfolders
// for a .sprsh declaring that name. The search is what rescues tracks
// saved before the .anif recorded sheet paths, and tracks whose recorded
// paths went stale because files were moved together.
//
// Every recorded sheet is loaded, needed or not (a prop's alternative
// sheet isn't "needed" but was loaded when the track was saved). missing
// lists needed names found nowhere; problems describes recorded or found
// .sprsh files that exist but couldn't be loaded (e.g. the image is gone).
func LoadSheetsForTrack(anifPath string, refs []SheetRef, needed []string) (sheets map[string]*editor.SpriteSheetTemplate, missing, problems []string) {
	sheets = map[string]*editor.SpriteSheetTemplate{}
	load := func(sprshPath string) {
		s, err := LoadSheetTemplate(sprshPath)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", sprshPath, err))
			return
		}
		if _, dup := sheets[s.Name]; !dup {
			sheets[s.Name] = s
		}
	}

	for _, ref := range refs {
		if _, err := os.Stat(ref.SprshPath); err == nil {
			load(ref.SprshPath)
		}
	}

	wanted := map[string]bool{}
	for _, n := range needed {
		if sheets[n] == nil {
			wanted[n] = true
		}
	}
	for _, ref := range refs {
		if sheets[ref.Name] == nil {
			wanted[ref.Name] = true
		}
	}
	if len(wanted) > 0 {
		for _, p := range findSprshByName(filepath.Dir(anifPath), wanted) {
			load(p)
		}
	}

	for _, n := range needed {
		if sheets[n] == nil {
			missing = append(missing, n)
		}
	}
	return sheets, missing, problems
}

// findSprshByName walks root (bounded by depth and entry count) and
// returns the .sprsh files whose declared name is in wanted. A .sprsh is
// named after its image file, which needn't match the sheet's name, so
// each candidate's name field is read rather than trusting the filename.
func findSprshByName(root string, wanted map[string]bool) []string {
	var found []string
	seen := 0
	rootDepth := strings.Count(filepath.Clean(root), string(filepath.Separator))
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entry: skip it, keep searching
		}
		if seen++; seen > sheetSearchMaxEntries {
			return filepath.SkipAll
		}
		if d.IsDir() {
			if path != root && (strings.HasPrefix(d.Name(), ".") ||
				strings.Count(filepath.Clean(path), string(filepath.Separator))-rootDepth > sheetSearchMaxDepth) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".sprsh") {
			return nil
		}
		var ts tomlSheetTemplate
		if _, err := toml.DecodeFile(path, &ts); err == nil && wanted[ts.Name] {
			found = append(found, path)
			delete(wanted, ts.Name)
		}
		if len(wanted) == 0 {
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
