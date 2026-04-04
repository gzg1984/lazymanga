# Normalization Framework Guide

This directory contains the post-index normalization pipeline for repository mode.
The target reader is future agents (or developers) who need to change normalization behavior quickly without re-reading the whole codebase.

## Where It Runs

Normalization is triggered from `backend/handlers/repo_normalize.go` after `repoisos` rows are rebuilt.

Flow:
1. Scan ISO files and build in-memory records (MD5 empty).
2. Rebuild `repoisos` table.
3. Start async normalization pipeline on inserted rows.

The API returns quickly after step 3 starts.

## Core Files

- `pipeline.go`: step interface, pipeline orchestration, step order, async execution.
- `os_relocation_step.go`: classify ISO type by filename keywords and relocate files.
- `md5_step.go`: calculate MD5 using the current (possibly relocated) path.

## Step Contract

```go
type RecordStep interface {
    Name() string
    Process(repoID uint, repoDB *gorm.DB, rootAbs string, record *models.RepoISO) error
}
```

Notes:
- `record` is a pointer. A step can mutate it for following steps.
- If a step changes filesystem path, it should update both:
  - database row (`repoisos`), and
  - `record.Path` / `record.FileName` in memory.

## Current Default Step Order

Defined in `newDefaultPipeline()` in `pipeline.go`:
1. `os-relocation`
2. `md5-backfill`

Order matters. MD5 runs after relocation so it hashes the final file path.

## Failure Behavior

Current behavior is tolerant:
- Per-step errors are logged.
- Pipeline continues with next step and next record.
- Failures are summarized in final pipeline logs.

This is useful for large batches, but keep in mind that one failed step does not stop later steps.

## Adding a New Normalization Step

1. Create a new file in this directory, for example `my_step.go`.
2. Implement `RecordStep`.
3. Register it in `newDefaultPipeline()` at the correct order.
4. Add clear logs for start/progress/failure with stable prefixes.

Minimal skeleton:

```go
type MyStep struct{}

func NewMyStep() RecordStep { return MyStep{} }
func (s MyStep) Name() string { return "my-step" }

func (s MyStep) Process(repoID uint, repoDB *gorm.DB, rootAbs string, record *models.RepoISO) error {
    // do work
    // optionally update DB and record
    return nil
}
```

## Safety Guidelines

- Keep all file operations within `rootAbs`.
- Prefer idempotent operations (safe to rerun).
- Handle cross-device moves (`rename` can fail with EXDEV).
- Avoid long blocking work in handlers; heavy work belongs in this async pipeline.

## Quick Validation

From `backend/`:

```bash
go test $(go list ./... | grep -v '^lazymanga/client$')
```

Runtime log keywords to watch:
- `ForceNormalizeRepo:`
- `NormalizePipeline:`
