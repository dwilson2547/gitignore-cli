# gitignore-cli

An interactive CLI tool for generating `.gitignore` files from the [github/gitignore](https://github.com/github/gitignore) template library.

Run it with no arguments to open the filterable TUI, or use the headless flags for automation and scripting.

---

## Usage

```
gitignore [flags]
```

### Flags

| Flag | Description |
|---|---|
| *(none)* | Open the interactive TUI to browse and select a template |
| `--list` | Print all available template names to stdout (one per line) |
| `--quiet <template>` | Apply a template non-interactively (no TUI) |
| `--version`, `-v` | Print the version and exit |
| `--help`, `-h` | Print help and exit |

---

## Interactive TUI

```bash
gitignore
```

Opens a filterable list of every template from the github/gitignore repository.  Type to narrow the list, use `↑`/`↓` to navigate, and press `Enter` to apply.

* Creates `.gitignore` if it does not exist.
* Appends to an existing `.gitignore` with a labelled section header.

---

## Headless flags

### List all templates

```bash
gitignore --list
```

Prints every available template name, one per line.  Useful for scripting, piping into `grep`, or letting an agent confirm a valid name.

```
Ada
Android
Autotools
...
Python
...
```

### Apply a template without TUI

```bash
gitignore --quiet Python
```

Fetches and applies the template to `.gitignore` in the current working directory.  Template name matching is case-insensitive.

```
Created .gitignore with Python template
```

If a `.gitignore` already exists, the new template is appended:

```
Appended Node template to .gitignore
```

Exit code is `0` on success, `1` on any error (unknown template, network failure, etc.).

---

## Caching

Template names are cached in `~/.cache/gitignore-cli/templates.json` for 24 hours, so repeated `--list` or `--quiet` calls are fast after the first fetch.

---

## Installation

```bash
go install gitignore-cli@latest
```

Or clone and build:

```bash
git clone https://github.com/dwilson2547/gitignore-cli
cd gitignore-cli
go build -o gitignore .
```

---

## Documentation

* [Headless flags — design and implementation notes](docs/headless-flags.md)

---

## Changelog

See [CHANGELOG.md](CHANGELOG.md).
