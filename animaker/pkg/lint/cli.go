package lint

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

// Run is the "animaker lint" command. Each argument is a glob matching
// one family's tracks, e.g. "art/human_*.anif"; families are checked
// separately. It prints findings to w and returns the exit code: 0 when
// there are no errors (info findings alone still pass), 1 when there are,
// 2 for bad usage.
func Run(args []string, w io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(w, `usage: animaker lint <glob> [<glob> ...]

Each glob is one character's family of tracks, checked for agreeing on
their props, e.g.:  animaker lint "art/human_*.anif" "art/city_guard_*.anif"
A glob matching .anichar files checks each character instead: its
animations are the family, and their markers must fall within them,
e.g.:  animaker lint "art/*.anichar"`)
		return 2
	}
	code := 0
	for _, glob := range args {
		paths, err := filepath.Glob(glob)
		if err != nil {
			fmt.Fprintf(w, "error: bad pattern %q: %v\n", glob, err)
			return 2
		}
		sort.Strings(paths)
		if len(paths) == 0 {
			fmt.Fprintf(w, "error: %s matches no files\n", glob)
			code = 1
			continue
		}
		chars, tracks := splitCharacters(paths)
		for _, c := range chars {
			code = max(code, report(w, c, "", Character(c)))
		}
		if len(tracks) > 0 {
			code = max(code, report(w, glob, fmt.Sprintf(" %d track(s),", len(tracks)), Family(tracks)))
		}
	}
	return code
}

// splitCharacters separates .anichar files from tracks.
func splitCharacters(paths []string) (chars, tracks []string) {
	for _, p := range paths {
		if strings.EqualFold(filepath.Ext(p), ".anichar") {
			chars = append(chars, p)
		} else {
			tracks = append(tracks, p)
		}
	}
	return chars, tracks
}

// report prints one family's findings - count is e.g. " 3 track(s)," -
// and returns its exit code.
func report(w io.Writer, name, count string, findings []Finding) int {
	fmt.Fprintf(w, "%s:%s %d finding(s)\n", name, count, len(findings))
	for _, f := range findings {
		fmt.Fprintf(w, "  %s\n", f)
	}
	if HasErrors(findings) {
		return 1
	}
	return 0
}
