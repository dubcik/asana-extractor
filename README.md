# Asana Extractor

A small Go application that periodically retrieves workspace users and projects
from Asana and writes each entity to its own formatted JSON file.

## Prerequisites

- Go 1.27 or later
- An Asana Personal Access Token (PAT)
- The GID of the Asana workspace to extract

## Configuration and PAT setup

The application reads configuration from environment variables. `ASANA_TOKEN`
and `ASANA_WORKSPACE_GID` are required. The PAT is not stored in this repository;
the client sends it using `Authorization: Bearer <token>`.

In PowerShell, enter the PAT without echoing it, set the workspace GID, and run
the application from the same terminal:

```powershell
$secureToken = Read-Host "Asana personal access token" -AsSecureString
$env:ASANA_TOKEN = [System.Net.NetworkCredential]::new("", $secureToken).Password
$env:ASANA_WORKSPACE_GID = "your_workspace_gid"
go run .
```

These variables remain set in that PowerShell session. They are available to
the application as plain process environment values, so do not print or expose
the process environment; close the terminal to discard them.

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `ASANA_TOKEN` | Yes | — | Asana Personal Access Token. |
| `ASANA_WORKSPACE_GID` | Yes | — | Workspace GID for users and projects requests. |
| `POLL_INTERVAL` | No | `5m` | Delay between scheduled extraction runs. Go duration syntax; for example `30s` or `5m`. |
| `ASANA_REQUESTS_PER_MINUTE` | No | `120` | Client-side maximum request rate, including retry attempts. |
| `HTTP_TIMEOUT` | No | `30s` | Timeout for each HTTP request. |
| `MAX_RETRIES` | No | `3` | Maximum additional attempts after retryable responses. |
| `OUTPUT_DIR` | No | `./output` | Root directory for extracted files. |

To poll every 30 seconds instead of the default 5 minutes:

```powershell
$env:POLL_INTERVAL = "30s"
go run .
```

To use the 5-minute interval explicitly:

```powershell
$env:POLL_INTERVAL = "5m"
go run .
```

## Output structure

Directories are created automatically. Each user and project is written as
readable, indented JSON in a separate file:

```text
output/
  users/
    <gid>.json
  projects/
    <gid>.json
```

An existing file for the same GID is overwritten on the next extraction.
Records absent from later API responses are not currently removed from disk.

## Pagination and request handling

The client uses the workspace-scoped Asana endpoints:

- `GET /workspaces/{workspace_gid}/users`
- `GET /workspaces/{workspace_gid}/projects`

Requests use `limit=100`. The client processes one page at a time, passes the
returned records to storage, then uses the exact opaque
`next_page.offset` value for the next request. It stops when `next_page` is
`null`; offsets are never calculated locally.

Requests are paced according to `ASANA_REQUESTS_PER_MINUTE` (120/minute by
default), including retries. On HTTP 429, the client waits the number of
seconds specified by `Retry-After` before retrying. HTTP 5xx responses are
retried with a bounded exponential backoff. Both retry paths are bounded by
`MAX_RETRIES`; ordinary permanent 4xx responses are returned without retrying.
Retry and pacing waits can be cancelled when the application receives Ctrl+C.

## Scheduling and shutdown

One extraction runs immediately at startup. The application then waits on one
ticker configured by `POLL_INTERVAL` before starting each subsequent run. It
does not run overlapping extractions. Press Ctrl+C to cancel in-flight HTTP
requests or waits and shut down gracefully.

## Architecture

The application is kept in a single Go package:

- `config.go` reads and validates environment configuration.
- `asana_client.go` handles authenticated HTTP requests, pagination, pacing,
  and retry behavior; page callbacks let the caller process results without
  retaining the full dataset.
- `app.go` coordinates user and project retrieval, per-entity storage, logging,
  and the polling loop.
- `storage.go` creates output directories and writes one formatted JSON file per
  entity.
- `models.go` contains the minimal user and project data structures.

## Tests

Run all tests from the repository root:

```powershell
go test ./...
```

Tests cover configuration defaults and validation, authentication and opaque
pagination, HTTP error and retry behavior, cancellable retry waits, and
per-entity JSON storage.
