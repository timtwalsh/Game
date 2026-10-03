// Package lint checks that a family of tracks - every .anif one character
// plays, e.g. human_walk, human_idle, human_attack_sword - agree on their
// props.
//
// Props are declared per track file, with no shared character manifest
// (docs/ANI_MAKER_SPEC.md, "Props"). A character's prop values live on the
// running instance and must keep applying when its state machine swaps one
// track for another; if two of its tracks disagree on a prop's name, that
// swap is exactly when a customization silently stops applying. Nothing in
// the file format prevents that drift, so this checks for it.
package lint

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"animaker/pkg/editor"
	"animaker/pkg/file"
)

// Severity is how serious a Finding is.
type Severity int

const (
	// Info is worth knowing but often intended, e.g. defaults that differ.
	Info Severity = iota
	// Error is a drift that will break customization at runtime.
	Error
)

func (s Severity) String() string {
	if s == Error {
		return "error"
	}
	return "info"
}

// Finding is one problem, in one file.
type Finding struct {
	Severity Severity
	File     string
	Message  string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s: %s: %s", f.Severity, f.File, f.Message)
}

// HasErrors reports whether any finding is an Error.
func HasErrors(fs []Finding) bool {
	for _, f := range fs {
		if f.Severity == Error {
			return true
		}
	}
	return false
}

// propInfo is one prop as declared by one track.
type propInfo struct {
	file string
	def  editor.PropDef
}

// Family checks that the tracks at paths - one character's - agree on
// their props, and that each track's nested-animation bindings name props
// that exist. A file that can't be read is an Error finding, not a
// failure, so one bad file doesn't hide the rest.
func Family(paths []string) []Finding {
	var out []Finding
	tracks := map[string]*editor.Track{}
	var files []string
	for _, p := range paths {
		t, _, err := file.LoadTrack(p)
		if err != nil {
			out = append(out, Finding{Error, p, fmt.Sprintf("can't read: %v", err)})
			continue
		}
		tracks[p] = t
		files = append(files, p)
	}
	sort.Strings(files)

	// Every prop name any track declares, and who declares it how.
	byName := map[string][]propInfo{}
	for _, f := range files {
		for _, pd := range tracks[f].Props {
			byName[pd.Name] = append(byName[pd.Name], propInfo{f, pd})
		}
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		decl := byName[name]
		declares := map[string]bool{}
		for _, d := range decl {
			declares[d.file] = true
		}
		for _, f := range files {
			if !declares[f] {
				out = append(out, Finding{Error, f, fmt.Sprintf("lacks prop %q, which %s declare%s",
					name, fileList(decl), plural(len(decl)))})
			}
		}
		var sheets, anims []propInfo
		for _, d := range decl {
			if d.def.IsAnimProp() {
				anims = append(anims, d)
			} else {
				sheets = append(sheets, d)
			}
		}
		if len(sheets) > 0 && len(anims) > 0 {
			out = append(out, Finding{Error, anims[0].file, fmt.Sprintf(
				"prop %q holds animations here but sprite sheets in %s", name, fileList(sheets))})
		}
		if len(decl) > 1 {
			for _, d := range decl[1:] {
				if d.def.Default != decl[0].def.Default {
					out = append(out, Finding{Info, d.file, fmt.Sprintf("prop %q defaults to %q; %s uses %q",
						name, short(d.def.Default), filepath.Base(decl[0].file), short(decl[0].def.Default))})
				}
			}
		}
	}

	nested := map[string]*editor.Track{}
	for _, f := range files {
		out = append(out, bindings(f, tracks[f], nested)...)
	}
	return out
}

// bindings checks one track's nested parts: each binding must name a prop
// the nested animation declares, and a passthrough a prop this track
// declares. nested caches nested tracks by path.
func bindings(f string, t *editor.Track, nested map[string]*editor.Track) []Finding {
	var out []Finding
	for _, part := range t.Parts {
		if part.Kind != editor.PartKindNestedAni || len(part.NestedBindings) == 0 {
			continue
		}
		child, ok := nested[part.NestedAniPath]
		if !ok {
			child, _, _ = file.LoadTrack(part.NestedAniPath)
			nested[part.NestedAniPath] = child // nil if unreadable, cached too
		}
		keys := make([]string, 0, len(part.NestedBindings))
		for k := range part.NestedBindings {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b := part.NestedBindings[k]
			if child != nil && child.FindProp(k) == nil {
				out = append(out, Finding{Error, f, fmt.Sprintf("part %q binds %q, which %s doesn't declare",
					part.Name, k, filepath.Base(part.NestedAniPath))})
			}
			if b.PassthroughFrom != "" && t.FindProp(b.PassthroughFrom) == nil {
				out = append(out, Finding{Error, f, fmt.Sprintf("part %q passes prop %q through to %q, but this track has no prop %q",
					part.Name, b.PassthroughFrom, k, b.PassthroughFrom)})
			}
		}
		if child == nil {
			out = append(out, Finding{Error, f, fmt.Sprintf("part %q nests %s, which can't be read, so its bindings weren't checked",
				part.Name, part.NestedAniPath)})
		}
	}
	return out
}

func fileList(ps []propInfo) string {
	names := make([]string, len(ps))
	for i, p := range ps {
		names[i] = filepath.Base(p.file)
	}
	return strings.Join(names, ", ")
}

func plural(n int) string {
	if n == 1 {
		return "s"
	}
	return ""
}

// short shows an .anif value by file name, a sheet by its name.
func short(v string) string {
	if editor.IsAnimValue(v) {
		return filepath.Base(v)
	}
	return v
}
