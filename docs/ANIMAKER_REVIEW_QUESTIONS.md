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

8. **Nested bindings UI (#21): open. Recommendation: switch to dropdowns
   now.** This question was unclear, so here it is restated. A nested
   animation (e.g. `torch.anif`) can have its own props, e.g. a `flame` prop
   that picks `flame_red` or `flame_blue`. A *binding* is how the parent track
   sets that prop for the copy it contains: either *passthrough* (copy the
   value of one of the parent's own props, so the character's `flame_color`
   drives the torch) or *static* (always `flame_red`). Today both the torch's
   prop name and the value are typed as free text. A typo quietly does
   nothing, and the torch falls back to its default. The question was: turn
   those boxes into dropdowns (the torch's props; then the parent's props, or
   the loaded sheets), as every other picker in the editor already is, or
   leave them until spec open question 1 is settled? Dropdowns are a small,
   self-contained change and are recommended. Say if you'd rather leave them.

9. **`Keyframe.ID` (#21): keep it an index, rename it to `Index`.** Stable
   keyframe IDs aren't worth adding:
   - Nothing outside the editor refers to keyframes: `.anif` files don't save
     a keyframe id, only `part_id` + `time_ms`.
   - A future game-side loader would identify keyframes by part and time.
   - The likeliest future reference, hit-spark events, is defined in the spec
     as firing at "a specific timeline instant", which is a time, not a
     keyframe.
   - Stable IDs would mean a new saved field plus migration, with no current
     consumer.

   Renaming the field to `Index` makes the code say what it actually is. It
   doesn't touch the file format, because the field isn't saved.

10. **Spec open questions / prop-schema linter: still open.** No answer yet on
    whether the linter gets its own issue.
