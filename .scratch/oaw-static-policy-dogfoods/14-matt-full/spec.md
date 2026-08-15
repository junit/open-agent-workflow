# MATT-FULL Dogfood Specification

## Domain

A Checklist is a Markdown document containing zero or more checklist Items. An
Item is complete only when its marker is `[x]` or `[X]`; `[ ]` is incomplete.
Other Markdown lines are context and do not become Items.

## Deliverable

Create a dependency-free Go command named `checklist` that accepts one Markdown
path and prints `<complete>/<total> complete`. It returns usage status `2` for
the wrong argument count and a non-zero status when the input cannot be read.

## Decisions

The parser owns the Checklist and Item domain behavior. The CLI owns file I/O,
argument validation, and output. No provider index, route, cache path, or
machine evidence participates in the behavior.
