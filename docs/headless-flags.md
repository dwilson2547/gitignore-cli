# Headless Flags

## Overview

`gitignore` ships with two headless flags that allow it to be used from automation scripts, CI pipelines, and AI agent skills without spawning the interactive TUI.

| Flag | Purpose |
|---|---|
| `--list` | Print every available template name to stdout, one per line |
| `--quiet <template>` | Fetch and apply a template non-interactively |

---

## `--list`

```bash
gitignore --list
```

Prints the full list of available `.gitignore` templates sourced from the [github/gitignore](https://github.com/github/gitignore) repository, one name per line.  Output respects the 24-hour local cache so repeated calls are fast.

### Example output

```
1C
1C-Bitrix
A-Frame
Actionscript
Ada
...
Python
...
```

### Agent use-case

An agent can capture the list as a variable and fuzzy-match it against a user's request to confirm a valid template name before calling `--quiet`.

---

## `--quiet <template>`

```bash
gitignore --quiet <template>
```

Applies `<template>` to the `.gitignore` in the current working directory without opening the TUI.

* **Case-insensitive** name resolution — `python`, `Python`, and `PYTHON` all work.
* Prints a single human-readable status line to stdout on success.
* Prints an error to stderr and exits non-zero on failure.

### Behaviour

| Condition | Result |
|---|---|
| No `.gitignore` exists | File is created with the template content |
| `.gitignore` exists (non-empty) | Template is appended under a clearly labelled section header |
| Template name not found | Error message with hint to run `--list`; exit code 1 |

### Example output (new file)

```
Created .gitignore with Python template
```

### Example output (appended)

```
Appended Node template to .gitignore
```

---

## Additional flags

| Flag | Purpose |
|---|---|
| `--version`, `-v` | Print the current version string and exit |
| `--help`, `-h` | Print usage information and exit |

---

## Implementation notes

### Design approach

The original codebase wrapped all network calls inside `tea.Cmd` closures that communicate results via message types (`templatesFetchedMsg`, `contentFetchedMsg`, `errMsg`).  These closures cannot be reused outside a Bubble Tea program.

To avoid duplicating logic, two plain (non-tea) helpers were extracted:

* `fetchTemplates() ([]string, error)` — reuses the existing `loadCache` / `saveCache` helpers so the 24-hour cache works identically in both modes.
* `fetchContent(name string) (string, error)` — downloads a single template by its canonical name.
* `resolveTemplate(templates []string, query string) (string, bool)` — case-insensitive lookup so agents do not need to know the exact capitalisation.

The existing `writeGitignore` function is used unchanged for the headless path.

The `main` function now inspects `os.Args` before constructing the Bubble Tea program; if a headless flag is detected the program exits before any TUI code runs.

### Exit codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Any error (network failure, template not found, I/O error) |

### No new dependencies

The headless flags are implemented purely with the Go standard library and the existing fetch/cache helpers.  No new modules were added to `go.mod`.
