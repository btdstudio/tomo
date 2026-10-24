# Tomo issues

Manage this project's issues. The backend comes from the effective settings
(run `tomo settings`, or `.tomo/bin/tomo settings`, when available; otherwise
read `~/.config/tomo/settings.json` and `.tomo/settings.json` directly, missing
files count as `{}`, project overlays global):

- `issues` — `github` (default; everything routes through the `gh` CLI) or
  `local` (a markdown issue store in `issuesDir`).
- `issuesDir` — where local issues live (default `issues`, relative to the
  project root).

Verbs (`$ARGUMENTS`):

- `new [--title <title>] [--labels <a,b>] [body]` — create an issue.
  - `github`: run `gh issue create --title "<title>" --body "<body>"` (add
    `--label <a> --label <b>` for each label). Print gh's output.
  - `local`: run `tomo issue new --title "<title>" --labels <a,b> --body
    "<body>"` (or `.tomo/bin/tomo issue new ...`). Print the created
    `<issuesDir>/<id>.md` path and `#<id>`.
- `list [--status open|closed]` — list issues.
  - `github`: run `gh issue list [--state open|closed]`.
  - `local`: run `tomo issue list` (or `.tomo/bin/tomo issue list`); print
    `#<id> [open|closed] <title>` per issue, open only by default, id order.
- `show <id>` — full detail for one issue.
  - `github`: run `gh issue view <id>`.
  - `local`: run `tomo issue show <id>` (or print the raw file); show
    frontmatter + body.

## Local store rules

One `<id>.md` per issue in `issuesDir`, with the controlled YAML-subset
frontmatter the helpers generate and parse (`id`, `status`, `title`,
`labels`) — never hand-parse arbitrary YAML. The next id is `max(numeric ids)
+ 1`. Never create over an existing id: if the target file already exists,
report it and stop. Skip non-numeric `.md` files in the store directory.

This command reads and writes issue data only. It never touches
`.tomo/state.json` and never starts or advances a phase.
