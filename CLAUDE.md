# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`his` is a CLI time tracking and log analysis tool written in Go that parses Markdown-formatted time logs and provides summaries by tags and titles.

### Log Format

The tool parses Markdown files with time entries in this format:

```markdown
## YYYYMMDD_HHMMSS TAG Title
```

Example:
```markdown
## 20250114_093000 DEV Implement feature X
## 20250114_103000 MTG Team standup
```

`###` headings may carry `TAG Nh` pairs before the title (`### 20250114_093000 DEV 1.5h MTG 0.5h Title`);
those hours count as written. Otherwise an entry's duration is the time difference between consecutive entries. Logs are aggregated by:
- **Tag only**: Groups all entries with the same tag per date
- **Tag + Title**: Groups entries with same tag and title per date

## Development Commands

### Building and Testing

```bash
# Full build pipeline (clean, gen, tidy, fmt, lint, build, test)
make all

# Build for current platform (Linux amd64)
make build

# Cross-platform builds
make build-linux-arm    # Linux ARM64
make build-darwin-amd   # macOS Intel
make build-darwin-arm   # macOS Apple Silicon
make build-windows-amd  # Windows amd64
make build-android-arm  # Android ARM64

# Install to $GOBIN
make install

# Run linting
make lint

# Format code
make fmt

# Run tests with coverage
make test

# Generate mocks
make gen

# Run specific commands
make latest arg="--count 5"
make tag arg="--count 10 --tag DEV"
```

### Development Workflow

1. **Before committing**: Run `make all` to ensure code passes all checks
2. **Testing**: Uses `gotestsum` for readable test output with atomic coverage mode
3. **Coverage threshold**: Enforced via `.testcoverage.yaml`
4. **Linting**: Only runs on new code (`new: true` in `.golangci.yaml`)

## Code Architecture

### Packages

- `cmd/` - Cobra commands (`root`, `latest`, `tag`, `gen`): args, config, output
- `internal/timelog/` - The time heading format (`### 20261030_130000 ALPHA 1h Title`).
  `Parse` and `Heading.String` are the only code that reads or writes it;
  `gen` writes through it and `summary` reads through it
- `internal/summary/` - Reads log files and aggregates by tag and tag+title
  - `Analyze()`: Entry point that finds files, parses logs, and generates summaries
  - `TimeLog.Fix()`: Calculates duration by setting end time from next entry's start
  - `NewResult()`: Aggregates logs into `TagSummaryLog` and `TagTitleSummaryLog`
- `internal/gen/` - Writes the daily headings from Claude Code sessions, rec's calls and tasks

### Commands

1. **`cmd/root.go`** - Cobra CLI root command (defaults to running `latest`)

2. **`cmd/latest.go`** - Shows most recent N time log entries
   - Default: 1 entry from last 2 files
   - Flags: `--count` (number of dates to show)

3. **`cmd/tag.go`** - Shows aggregated summaries by tag or tag+title
   - Default: 14 days from last 2 files
   - Flags: `--count` (days), `--tag` (filter by tag)

### Data Flow

```
Markdown files (~/pattern/*.md)
  ↓ findRecentryFiles() - glob pattern matching
  ↓ parseFiles() - iterate files
  ↓ parseFile() - timelog.Parse per line
  ↓ newTimeLogs() - create TimeLog structs
  ↓ TimeLog.Fix() - calculate durations
  ↓ filter by Valid() - remove incomplete logs
  ↓ NewResult() - aggregate into summaries
  ↓ PrintTagResult() / PrintTagTitleResult()
```

### Configuration

- **Config file**: `~/.his` (YAML format)
- **Pattern key**: `pattern: "~/*.md"` specifies log file search glob

Example `~/.his`:
```yaml
pattern: "~/logs/*.md"
```

## Testing Strategy

- **Test runner**: `gotestsum` with `testname` format for clean output
- **Coverage mode**: `atomic` for safe concurrent testing
- **Coverage validation**: Automatic threshold checking via `go-test-coverage`
- **Mocking**: `mockery` generates mocks from interfaces (see `.mockery.yaml`)

## CI/CD

### Release Workflow (`.github/workflows/release.yml`)

Triggered on version tags (`v*.*.*`):

1. Checkout code
2. Setup Go environment
3. Run `golangci-lint` (v2.5.0)
4. Build and release via `GoReleaser`

### GoReleaser Configuration

Builds for:
- Linux amd64
- Darwin (macOS) amd64 + arm64
- Android arm64

Produces:
- `tar.gz` archives (except Windows)
- Checksums
- GitHub releases

## Code Quality

### Linters Enabled
- `gosec` - Security checks
- `govet` - Standard Go vet
- `staticcheck` - Advanced static analysis

### Linter Settings
- **Auto-fix**: Enabled (`fix: true`)
- **New code only**: Only checks uncommitted changes (`new: true`)
- **No tests**: Skips test files (`tests: false`)
- **Timeout**: 10 minutes

## Development Notes

### Adding New Commands

1. Create new file in `cmd/` (e.g., `cmd/mycommand.go`)
2. Use Cobra command structure:
```go
var myCmd = &cobra.Command{
    Use:   "my",
    Short: "Description",
    Run: func(cmd *cobra.Command, args []string) {
        // Implementation
    },
}

func init() {
    rootCmd.AddCommand(myCmd)
}
```
3. Add Makefile target for convenience

### Extending Summary

The summary package follows a functional pipeline pattern:
- Add new summary types by implementing `LogBaser` interface
- New aggregation logic goes in `NewResult()`
- Output formatting via `String()` method on summary types

### Time Calculation Logic

Duration is calculated by setting each log's `End` time to the next log's `Start` time:
```go
func (t *TimeLog) Fix(tl *TimeLog) {
    t.End = tl.Start
    t.Sec = t.End - t.Start
}
```

Logs without a following entry (last entry in file) are marked invalid and filtered out.
