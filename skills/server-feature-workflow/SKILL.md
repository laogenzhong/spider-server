---
name: server-feature-workflow
description: Use this when adding or modifying spider-server features driven by the spider client, especially protobuf APIs, MySQL models, gRPC routers, configuration-backed behavior, service startup settings, secrets, addresses, timeouts, feature flags, and full/incremental restore sync tasks. Follow this workflow to inspect client changes, put configurable values in common/config plus config.yaml instead of hardcoding them, update proto/model/router registration, integrate restore synchronization, regenerate protobuf code, and verify with Go tests.
---

# Spider Server Feature Workflow

Use this skill when the user asks to add server support for a client-side feature, update proto contracts, make a new data domain participate in full/incremental restore, or add code that needs runtime settings.

## First Read

1. Check server state:
   - `git status --short`
   - relevant files under `proto/primary`, `mysql/model`, `game/router`, `game/routers.go`, `mysql/a_register_mysql.go`
   - config files: `common/config/config.go`, `config.yaml`, `.gitignore`
2. If the request references the client project, inspect `/Users/huitailang/workdir/spider`:
   - `git -C /Users/huitailang/workdir/spider status --short`
   - `rg` for the domain terms in Swift files, for example `photo`, `weight`, `tag`, `sync`, `snapshot`
3. Do not overwrite unrelated dirty changes. Treat existing modifications as user work unless clearly created by this task.

## Configuration Workflow

Never scatter runtime values directly in feature code. If a value may differ by machine, environment, deployment, test setup, or future tuning, put it in config first.

Use config for:

- addresses, ports, hosts, URLs, paths
- database credentials and connection pool settings
- secrets, salts, token TTLs, nonce TTLs
- timeouts, intervals, cleanup durations
- feature flags and enable/disable switches
- auth public method prefixes and similar allowlists
- batch sizes, limits, retention windows, log settings

Keep true protocol/domain constants in code, such as protobuf enum values, fixed binary framing bytes, stable task IDs, DB column sizes, and algorithm-required lengths.

When adding a configurable value:

1. Add a field to the appropriate struct in `common/config/config.go`, or create a focused nested config struct if it is a new domain.
2. Add a default in `Default()`.
3. Add normalization in `Normalize()` if an empty value should fall back to the default.
4. Add a typed helper when parsing is needed, for example `TimeoutDuration()` for duration strings.
5. Add the same key to `config.yaml`.
6. Wire code through the loaded `appconfig.Config` from `cmd/main.go`, or through a focused `Configure...` function when a package-level manager/interceptor needs runtime settings.
7. For local/private overrides, use `SPIDER_SERVER_CONFIG=config.local.yaml`; keep `config.local.yaml` ignored by git.

Before finishing, scan for newly introduced hardcoded runtime values outside config:

```bash
rg -n "localhost|127\\.0\\.0\\.1|:[0-9]{2,5}|secret|password|timeout|time\\.Second|time\\.Minute|time\\.Hour" \
  -g '*.go' -g '!download/**' -g '!downloads/**' -g '!autodownload/**' -g '!delgif/**' -g '!dgitfname/**'
```

It is acceptable for this scan to find fallback defaults inside `common/config/config.go` and protocol constants in generated or low-level framing code.

## Server Feature Checklist

For a new data domain, implement these layers in order:

1. **Config**
   - Decide whether the feature needs runtime settings.
   - If it does, add those settings through the Configuration Workflow before wiring the feature code.
   - Do not add new secrets, addresses, ports, paths, timeouts, intervals, or feature flags directly in router/model/startup code.

2. **Proto**
   - Add or update `proto/primary/<domain>.proto`.
   - Keep `package api;` and `option go_package = "spider/api;api";` unless the surrounding proto says otherwise.
   - Define record messages, save/delete request and response messages, and a focused service for direct CRUD.
   - For index-only features such as iOS photos, store references and metadata, not binary blobs, unless the user explicitly asks for upload/storage.

3. **Model**
   - Add `mysql/model/<domain>_model.go`.
   - Include `ID`, `UID`, domain identity fields, `CreatedAt`, `UpdatedAt`, and `gorm.DeletedAt`.
   - Prefer idempotent saves with `clause.OnConflict` using a stable client identifier when the client can provide one.
   - Use soft delete for deletions so restore/incremental sync can propagate tombstones.

4. **Router**
   - Add `game/router/<domain>_api.go`.
   - Get `uid` from `session.GetUser(ctx).UID()`.
   - Validate required fields before model calls.
   - Convert model structs to pb structs with local converter helpers.
   - Return business errors using `session.Error(ctx, gamecode.Xxx, &pb.SomeResponse{})`, matching `skills/spider-router-error-codes`.

5. **Registration**
   - Register the model in `mysql/a_register_mysql.go`.
   - Register the gRPC service in `game/routers.go`.

## Restore And Incremental Sync

The server uses `ClientRestoreService` as a unified full/incremental sync API.

Client flow:

1. Call `GetRestorePlan(start_snapshot_id, preferred_batch_size)`.
2. `start_snapshot_id = 0` means full restore.
3. Nonzero `start_snapshot_id` means incremental sync since the previous saved `end_snapshot_id`.
4. Server returns `end_snapshot_id = current server time`.
5. If `is_latest = true`, client can save `end_snapshot_id`.
6. Otherwise fetch each `RestoreTask` with `FetchRestoreBatch(start_snapshot_id, end_snapshot_id, task_id, batch_index, batch_size)`.
7. After all tasks are fetched, client saves `end_snapshot_id`.

When adding a new synced domain:

1. Update `proto/primary/restore.proto`:
   - Add a `RestoreDataType` enum value.
   - Add `<Domain>SyncItem` with `record`, `deleted`, `deleted_at`, `changed_at`.
   - Add `<Domain>RestoreBatch` with repeated `items`.
   - Add the batch to `RestoreBatchResponse.payload`.
2. Add model functions:
   - `Count<Domain>Changes(uid, startSnapshotID, endSnapshotID)`.
   - `List<Domain>ChangesPage(uid, startSnapshotID, endSnapshotID, limit, offset)`.
   - Use `db.Unscoped()` so soft-deleted rows are visible.
   - Full restore (`startSnapshotID <= 0`) should include rows that existed at `endSnapshotID`: `created_at <= end AND (deleted_at IS NULL OR deleted_at > end)`.
   - Incremental sync should include created, updated, or deleted rows within `(start, end]`.
3. Update `game/router/restore_api.go`:
   - Add a stable `task_id` constant.
   - Include the domain in `GetRestorePlan`.
   - Add a `FetchRestoreBatch` switch branch that returns sync items with delete metadata.

## Protobuf Generation

Run:

```bash
./gen.sh
```

Do not run `gofmt` on `.proto` files. Format only Go files.

Note: `/gen/` is ignored in this repo, but generation should still pass so server code compiles against the new pb types.

## Verification

Run focused tests first:

```bash
GOCACHE=/private/tmp/spider-server-go-build go test ./gen/... ./mysql/model ./game/router ./mysql
```

Then run full tests:

```bash
GOCACHE=/private/tmp/spider-server-go-build go test ./...
```

If a failure is unrelated, report it clearly with the exact package/test and keep the feature-specific verification result separate.

## Final Response

Summarize:

- Proto files changed.
- Model/router files added or updated.
- Config fields added or reused, including `config.yaml` keys.
- Restore sync participation, if any.
- Registration changes.
- Test commands and results.

Mention important product boundaries, for example: photo index sync does not restore image binary data unless upload/storage is added.
