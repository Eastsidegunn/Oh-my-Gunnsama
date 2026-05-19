# Test Spec: Remote Observer Rebuild

## Completion claim
`remote/` is a standalone TDD-built observer wrapper that scans a fixed project read-only, writes tablet-friendly report files, and optionally serves them.

## Unit tests
1. Scanner
   - non-git directory returns degraded report with warning.
   - git repo with changed file returns branch, dirty status, changed file list, diff stat.
   - scanner command allowlist contains only read-only git commands.
2. Report renderer
   - writes `state.json` with expected fields.
   - writes `index.html` containing report cards and references state.
   - escapes HTML content from file names/status text.
3. Server
   - serves `/` and `/state.json`.
   - refreshes report via injected scanner on interval or explicit handler path.
4. CLI
   - `remote observe --once --project <dir> --out <dir>` writes files.
   - missing project fails.
   - default host is loopback for serve mode.

## Integration verification
- `cd remote && go test ./...`
- `cd remote && go build ./...`
- `cd remote && go vet ./...`
- smoke: `cd remote && go run ./cmd/remote observe --once --project .. --out /tmp/remote-report`

## Boundary checks
- Root `go test ./...` still passes.
- Root `cmd/omg` does not import remote packages.
- No code path edits observed project unless output directory is explicitly inside it.
- No tmux/cmux/codex command injection.

---

## RALPLAN iteration 2 additions

### Read-only output tests
- `--out` inside `--project` fails.
- default output path is outside project.
- observer scan does not create `.omg/control-room` or any file under project.

### CLI mode tests
- no mode flag behaves as once and exits after writing report.
- `--once --serve` fails.
- `--serve --refresh 500ms` fails because refresh is below minimum.
- default serve host is `127.0.0.1`.

### Safe git runner tests
- runner exposes allowed argv forms and uses no shell.
- commands are executed as argv with `git`, `-C`, absolute project path, fixed args.
- project paths with spaces and leading dashes are passed safely as argv.
- command timeout returns warning/degraded report instead of hanging.

### State schema tests
- `state.json` has schema_version, generated_at, project_path, git, cards, risks, suggested_commands, warnings.
- non-git directory renders degraded state and HTML.

### Removal tests
- `remote control-room start` is not a valid command.
- final tree has no `remote/internal/controlroom` package.
- no code references `/api/messages`, queues, token pairing, or `.omg/control-room`.
