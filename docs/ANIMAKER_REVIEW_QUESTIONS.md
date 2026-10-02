# Animaker review: questions and decisions

A full review of `animaker/` was done on 2026-10-01: every source file in `pkg/editor`,
`pkg/file`, `pkg/ui`, `pkg/app` and `pkg/applog`, plus `go vet` and `go test`.
Clear defects were filed as GitHub issues (index below). This file records the
design decisions the code couldn't settle on its own, as answered by the owner;
the matching issues carry the same decision in a comment.

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
| [#22](https://github.com/timtwalsh/Game/issues/22) | Low | Scrubbing should clear the keyframe selection (decision 4 below) |
| [#23](https://github.com/timtwalsh/Game/issues/23) | Medium | Nested bindings editor: dropdowns, passthrough or static (decision 8 below) |
| [#24](https://github.com/timtwalsh/Game/issues/24) | Medium | Prop-schema linter (decision 10 below) |

## Decisions (answered 2026-10-02)

1. **Deleting a prop that parts still use (#13): refuse.** Do it the same way as
   removing a sheet: refuse the delete and list the parts that use the prop.
   Also clear the prop's `PreviewProps` entry when the delete goes through.

2. **Undo scope (#12): track only, for now.** Importing or removing a sheet,
   and preview overrides, stay out of undo.

3. **Typed-field undo (#12): one step per focus burst.** Click into a field,
   type, then leave it: that is one undo step, however many keystrokes it took.

4. **Selected keyframe vs playhead: clear on scrub.** Scrubbing clears the
   keyframe selection, so the fields always edit what the canvas shows: the
   keyframe at the playhead, or a new one if the artist edits a pose that
   isn't keyed there. Filed as #22.

5. **Platforms (#16): Windows required, Linux nice to have.** Skip the
   crash-log message-line assertion on non-Windows. CI runs Animaker tests on
   `windows-latest`; an `ubuntu-latest` run is optional, once the skip is in.

6. **Re-importing an image that has a `.sprsh` (#15): replace after
   confirmation.** Prefill the dialog from the existing template. If the
   artist changes it, they must explicitly accept that the existing sheet will
   be replaced before the import goes ahead.

7. **Nested-animation cache (#17): reload every time.** Opening a track
   reloads its nested `.anif` files from disk.

8. **Nested bindings UI: dropdowns; each binding is passthrough or static.**
   A parent track either passes one of its own props through to the nested
   animation, or pins a fixed value. For example, `city_guard_torch_walk.anif`
   pins `torch.torch_base = torchbase_metal`, while `player_torch_walk.anif`
   declares its own `torch_base` prop and passes it through. The data model
   already supports exactly this (`PropBinding`), so only the editor UI changes:
   free text becomes dropdowns. Filed as #23, together with a related gap: a
   sheet named only by a static binding isn't counted as used.

9. **`Keyframe.ID`: keep it as the keyframe's position; rename it to `Index` (#21).**
   Stable keyframe IDs aren't worth adding: nothing refers to a keyframe except
   by part and time, and the field isn't saved, so the rename doesn't change the
   file format.

10. **Prop-schema linter: yes, filed as #24.** One open point: how a "family"
    of tracks is defined. The recommendation is an explicit glob per family,
    because splitting on the first `_` breaks names like `city_guard_*`.
    Spec open questions 1–3 are still unresolved.
