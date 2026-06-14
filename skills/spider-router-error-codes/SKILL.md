---
name: spider-router-error-codes
description: Maintain spider-server router API error handling. Use when adding or changing Go router logic under spider-server/game/router, adding business validation, database failure handling, auth/session-facing API behavior, or error codes; ensure errors follow sign_api.go/session.Error trailer status_code style instead of direct grpc status.Error/status.Errorf returns.
---

# Spider Router Error Codes

## Purpose

Keep `spider-server` router APIs consistent with `game/router/sign_api.go`: business failures return an empty typed protobuf response plus `session.Error(ctx, gamecode.Xxx, &pb.Response{})`, which sets the `status_code` trailer and returns a nil gRPC error.

## Workflow

1. Inspect the existing pattern before editing:
   - `game/router/sign_api.go`
   - `game/session/session.go`
   - `game/code/error_code.go`
   - The target `game/router/*.go` file.
   - For admin-only routers, use `game/router/admin_<domain>_api.go` and `Admin<Domain>Api`; keep `admin` first in filenames and type names.

2. For every new or changed router failure path:
   - Do not return `status.Error`, `status.Errorf`, `codes.InvalidArgument`, `codes.Internal`, or generic `fmt.Errorf` for business/API failures.
   - Return `session.Error(ctx, gamecode.SomeCode, &pb.SomeResponse{})`.
   - Use the exact response type of the handler, including `&emptypb.Empty{}` where appropriate.
   - Preserve successful not-found-as-empty-result behavior when the API already intentionally returns success, such as `Exists: false`.

3. Add missing business error codes in `game/code/error_code.go`:
   - Keep existing constants and values stable.
   - Add descriptive names grouped by feature/module.
   - Use non-overlapping numeric ranges. Current convention:
     - `100xx`: sign/auth
     - `200xx`: weight records
     - `300xx`: training tags/workout tag bindings
     - `400xx`: restore/sync
     - `500xx`: friend APIs
     - `600xx`: body photos
   - Add a short Chinese comment for each exported constant, matching the existing style.

4. Keep imports clean:
   - Add `gamecode "spider-server/game/code"` where the router needs error codes.
   - Keep `spider-server/game/session`.
   - Remove unused `google.golang.org/grpc/codes` and `google.golang.org/grpc/status` imports after replacing direct gRPC errors.

5. Validate before finishing:
   - Run `gofmt` on changed Go files.
   - Run `rg -n "status\\.Error|status\\.Errorf|codes\\." game/router game/code` and confirm no router business error paths remain.
   - Run `go test ./...`.
   - Check `git diff --check` for whitespace issues.

## Notes

- The error code package may have a package name different from the import alias; follow existing imports and use `gamecode` as the local alias.
- Do not change unrelated dirty files. The repo may already contain user changes.
- If an interceptor returns auth/session errors, use existing session/auth error helpers and codes rather than inventing router response types.
