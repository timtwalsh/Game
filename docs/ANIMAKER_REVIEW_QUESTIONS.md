# Animaker review: open questions

A full review of `animaker/` was done on 2026-10-01: every source file in `pkg/editor`,
`pkg/file`, `pkg/ui`, `pkg/app` and `pkg/applog`, plus `go vet` and `go test`.
Clear defects were filed as GitHub issues (index below). This file collects the
**decisions the code can't settle on its own**. Each one is a design choice for
the owner, not a bug. Answer inline, or delete an entry once it's decided.

## Issues filed

| # | Severity | Summary |
|---|---|---|
| [#10](https://github.com/timtwalsh/Game/issues/10) | High | Undo reverts two actions; Redo loses the latest; history gets corrupted after an undo |
| [#11](https://github.com/timtwalsh/Game/issues/11) | High | No unsaved-changes prompt (close / New / Open discard work) |
| [#12](https://github.com/timtwalsh/Game/issues/12) | Medium | Many edits aren't undoable (typed fields, nudges, fixed sheet, bindings) |
| [#13](https://github.com/timtwalsh/Game/issues/13) | Medium | Deleting a prop leaves parts linked to a missing prop, drawing nothing |
| [#14](https://github.com/timtwalsh/Game/issues/14) | Medium | Add Part dialog drops the prop for nested parts; invalid combinations accepted |
| [#15](https://github.com/timtwalsh/Game/issues/15) | Medium | Import Sprite Sheet: no validation, silent overwrites, ignores an existing .sprsh |
| [#16](https://github.com/timtwalsh/Game/issues/16) | Medium | CI never runs Animaker tests; applog crash test fails on Linux |
| [#17](https://github.com/timtwalsh/Game/issues/17) | Low–Med | Open/New keep stale preview overrides, nested-anim cache, transport controls |
| [#18](https://github.com/timtwalsh/Game/issues/18) | Low | File robustness: Windows paths in .sprsh, unsorted keyframes, non-atomic save |
| [#19](https://github.com/timtwalsh/Game/issues/19) | Low | Per-frame rebuild of all canvas/timeline objects (perf, unmeasured) |
| [#20](https://github.com/timtwalsh/Game/issues/20) | Low | `map/objects/animaker.md` says nested animations don't render (stale) |
| [#21](https://github.com/timtwalsh/Game/issues/21) | Low | Small polish items |

## Questions

1. **Deleting a prop that parts still use (#13): refuse, or unlink?**
   Removing a sheet *refuses* and lists what uses it. For props, the
   alternative is to unlink the parts and keep their `FixedSheet`, which
   `DropTile` always fills in, so they'd keep drawing the art they show now.
   Which do you want? Unlinking is friendlier. Refusing matches how sheets work.

2. **What should undo cover (#12)?** Today it snapshots only the `Track`.
   Should editor-session actions that change what gets *saved* also be
   undoable? The main ones are importing a sheet and removing one: both change
   the `.anif`'s `[[sheets]]` table, but they live in `Project.LoadedSheets`.
   And should preview overrides stay out of undo, as they are now?

3. **Typed-field undo granularity (#12).** Is one undo step per *focus burst*
   (click into X, type `-12.5`, leave the field = one step) what you want, or
   per committed value (Enter / leaving the field)?

4. **Selected keyframe vs playhead.** After you click a keyframe marker and then
   scrub elsewhere, the Selected Keyframe fields still edit *that* keyframe,
   while the canvas shows the interpolated pose at the playhead. The panel
   labels it with "@ Nms", so it's discoverable, but a canvas drag edits the
   keyframe *at the playhead* instead. Should scrubbing clear the keyframe
   selection, so the fields always match what the canvas shows? I left this as
   is, since it may be intentional.

5. **Crash-log message line on Linux and macOS (#16).** Is Windows the only
   platform that matters for the editor? If so, the fix is to skip that one
   assertion on other platforms. If not, stderr should be captured properly on
   Unix (`dup2` onto fd 2). And should CI run Animaker tests on
   `windows-latest`, `ubuntu-latest`, or both?

6. **Re-importing an image that already has a `.sprsh` (#15).** Should the
   import dialog prefill from the existing template and *refuse* to change the
   grid, given the spec's "new layout = new name, never reorganized in place"?
   Or should it allow the change with a warning?

7. **Nested-animation cache (#17).** Should reopening a track reload nested
   `.anif` files from disk every time (simplest, always fresh), or only when
   the file's modification time has changed?

8. **Nested bindings UI (#21).** The free-text binding editor predates the
   pick-list conventions everywhere else. Should it become pick-lists (the
   nested track's props on the left; the parent's props, or the values the
   nested prop allows, on the right)? Or is the bindings feature likely to be
   redesigned along with spec open question 1 (one prop fanning out to
   several sheets), in which case it's not worth polishing yet?

9. **`Keyframe.ID` is just the index.** `normalizeKeyframes` renumbers IDs to
   slice positions after every sort, and `Selection.KeyframeIndex` is also an
   index. That works, but the name suggests a stable identity it doesn't have.
   (Parts *do* have stable IDs.) Is it worth making keyframe IDs stable before
   anything else (such as a game-side loader or an event system) starts
   referring to keyframes, or is "index" the intended meaning? If so, consider
   renaming it.

10. **Spec open questions are untouched.** `docs/ANI_MAKER_SPEC.md` "Open
    questions" 1–3 (prop → several sheets, a sword's "bent state",
    hit-spark events) are still unresolved in code, as the spec says. Also
    still unbuilt: the prop-schema linter the spec calls "load-bearing, not a
    nice-to-have". Should that linter get its own issue now?
