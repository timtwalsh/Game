package lint

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
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
their props, e.g.:  animaker lint "art/human_*.anif" "art/city_guard_*.anif"`)
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
		findings := Family(paths)
		fmt.Fprintf(w, "%s: %d track(s), %d finding(s)\n", glob, len(paths), len(findings))
		for _, f := range findings {
			fmt.Fprintf(w, "  %s\n", f)
		}
		if HasErrors(findings) {
			code = 1
		}
	}
	return code
}
