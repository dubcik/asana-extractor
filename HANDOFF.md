# Asana Extractor — Handoff History

This file preserves the important project context for continuing work in a new
chat. It summarizes the implementation history and current behavior; it does
not contain credentials.

## Project

A small, dependency-free Go application that retrieves users and projects from
an Asana workspace, writes one JSON file per entity, and repeats on a
configurable schedule.

## Implementation history

1. **Foundation**
   - Added a single `main` package with configuration, minimal user/project
     models, an application skeleton, and configuration tests.
   - Required configuration: `ASANA_TOKEN` and `ASANA_WORKSPACE_GID`.
   - Defaults: `POLL_INTERVAL=5m`, `ASANA_REQUESTS_PER_MINUTE=120`,
     `HTTP_TIMEOUT=30s`, `MAX_RETRIES=3`, and `OUTPUT_DIR=./output`.

2. **Asana HTTP client**
   - Added authenticated workspace endpoints:
     `/workspaces/{workspace_gid}/users` and
     `/workspaces/{workspace_gid}/projects`.
   - Uses `Authorization: Bearer <token>`; the PAT is read from the environment
     and must never be put in source code or committed configuration.
   - Requests pages with `limit=100`, passing through Asana's opaque
     `next_page.offset` and stopping when `next_page` is null.
   - Streams one page at a time through callbacks.
   - Adds request pacing, `Retry-After` handling for 429, bounded retries for
     429 and 5xx, and cancellable waits.

3. **Storage and scheduler**
   - Saves users to `OUTPUT_DIR/users/<gid>.json` and projects to
     `OUTPUT_DIR/projects/<gid>.json`, formatted as readable JSON.
   - Creates output directories and validates entity GIDs before using them in
     file paths.
   - Runs an extraction immediately, then uses one ticker configured with
     `POLL_INTERVAL`. Ctrl+C cancels the application context and stops
     in-flight HTTP requests and waits.

4. **Documentation and final review**
   - README documents prerequisites, environment configuration and PowerShell
     PAT setup, run commands, output layout, pagination, rate limits/retries,
     polling, architecture, and tests.
   - Final validation completed successfully with `gofmt`, `go test ./...`,
     `go test -count=1 ./...`, and `go build ./...`.

## Running the application

In PowerShell, set the token without echoing it, set the workspace GID, then run
from the project directory:

```powershell
$secureToken = Read-Host "Asana personal access token" -AsSecureString
$env:ASANA_TOKEN = [System.Net.NetworkCredential]::new("", $secureToken).Password
$env:ASANA_WORKSPACE_GID = "your_workspace_gid"
go run .
```

Set an alternate polling interval before starting, if needed:

```powershell
$env:POLL_INTERVAL = "30s"
```

The default is five minutes. Files are written under `OUTPUT_DIR`, which
defaults to `./output` relative to the working directory from which the process
is launched. `OUTPUT_DIR` can be set to use another location.

## Existing output

At the time of the latest output check, JSON files existed in:

- `output/users/` — four user files
- `output/projects/` — two project files

This is only a snapshot; subsequent extractions can update existing files or
add more files.

## Tests

Run from the repository root:

```powershell
go test ./...
```

Tests cover configuration, authentication, pagination, HTTP errors, rate-limit
and server-error retries, retry cancellation, and JSON storage.

## Known caveats and open decisions

- Files for entities absent from later Asana responses are not deleted.
- Models retain the fields currently defined in `models.go`; if full API
  records must be preserved, extend the models or store additional response
  fields.
- Retry/pagination/storage behavior has focused tests. The scheduler and
  end-to-end signal shutdown do not have dedicated integration tests.
- No PAT value is intentionally stored in this handoff or in source files.
