---
name: server-feature-workflow
description: Use this when adding or modifying spider-server features driven by the spider client, especially protobuf APIs, MySQL models, gRPC routers, and full/incremental restore sync tasks. Follow this workflow to inspect client changes, update proto/model/router registration, integrate restore synchronization, regenerate protobuf code, and verify with Go tests.
---

# Spider Server Feature Workflow

Use this skill when the user asks to add server support for a client-side feature, update proto contracts, or make a new data domain participate in full/incremental restore.

## First Read

1. Check server state:
   - `git status --short`
   - relevant files under `proto/primary`, `mysql/model`, `game/router`, `game/routers.go`, `mysql/a_register_mysql.go`
2. If the request references the client project, inspect `/Users/huitailang/workdir/spider`:
   - `git -C /Users/huitailang/workdir/spider status --short`
   - `rg` for the domain terms in Swift files, for example `photo`, `weight`, `tag`, `sync`, `snapshot`
3. Do not overwrite unrelated dirty changes. Treat existing modifications as user work unless clearly created by this task.

## Server Feature Checklist

For a new data domain, implement these layers in order:

1. **Proto**
   - Add or update `proto/primary/<domain>.proto`.
   - Keep `package api;` and `option go_package = "spider/api;api";` unless the surrounding proto says otherwise.
   - Define record messages, save/delete request and response messages, and a focused service for direct CRUD.
   - For index-only features such as iOS photos, store references and metadata, not binary blobs, unless the user explicitly asks for upload/storage.

2. **Model**
   - Add `mysql/model/<domain>_model.go`.
   - Include `ID`, `UID`, domain identity fields, `CreatedAt`, `UpdatedAt`, and `gorm.DeletedAt`.
   - Prefer idempotent saves with `clause.OnConflict` using a stable client identifier when the client can provide one.
   - Use soft delete for deletions so restore/incremental sync can propagate tombstones.

3. **Router**
   - Add `game/router/<domain>_api.go`.
   - Get `uid` from `session.GetUser(ctx).UID()`.
   - Validate required fields before model calls.
   - Convert model structs to pb structs with local converter helpers.
   - Return `InvalidArgument`, `NotFound`, or `Internal` using the style of existing routers.

4. **Registration**
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
- Restore sync participation, if any.
- Registration changes.
- Test commands and results.

Mention important product boundaries, for example: photo index sync does not restore image binary data unless upload/storage is added.
