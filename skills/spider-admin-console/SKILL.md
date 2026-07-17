---
name: spider-admin-console
description: Maintain the spider-server Vue 3 Admin console and its Go/local APIs. Use when adding or changing admin-console pages, admin HTTP endpoints, payments, refunds, Pro management, daily activity, registrations, local offer-code management, or any paginated Admin list; enforce newest-first stable pagination and preserve existing client/server behavior.
---

# Spider Admin Console

## Scope

Work primarily in:

- `admin-console/src` for Vue UI and API calls.
- `admin-console/local-offer-reply.js` for the fully local offer-code service.
- `gateway/admin_console.go` and `mysql/model/admin_console_model.go` for remote Admin APIs and queries.

Keep local offer-code data local. Keep remote requests behind the existing local HMAC-signing proxy and do not expose the shared secret to browser JavaScript.

## Pagination Order

Every time-based paginated Admin list must show the newest records first across the entire result set.

1. Sort in the database or service before applying `LIMIT`/`OFFSET` or slicing pages. Never fetch ascending pages and reverse only the Vue array.
2. Use the list's user-visible business time as the primary descending key:
   - Payments: `purchase_at DESC`.
   - Refund requests: `notification_signed_at DESC`.
   - Completed refunds: `revocation_at DESC`.
   - Current daily activity: `last_app_enter_at DESC`.
   - Historical activity: `activity_date DESC`, then `last_app_enter_at DESC`.
   - Registrations: `created_at DESC`.
3. Add a unique, deterministic descending tie-breaker, normally `id DESC`.
4. Apply filters and search before counting and paging. Page 1 must always contain the records closest to the current time.
5. Return empty paginated collections as `items: []`, never `items: null`.
6. When adding a new paginated list, identify its business timestamp explicitly and add a test that covers order across at least two pages. Include equal timestamps when practical to verify the tie-breaker.

The local offer-code library is the explicit exception: redemption is sequential, so filter first and then sort by sequence `id ASC` before pagination. Keep `全部 / 未兑换 / 已兑换` filters; the first row under `未兑换` must be the next code to issue.

Do not add another ascending Admin pagination order unless the user explicitly requests an exception for that specific screen.

## Read-Only Daily Metrics

For Admin daily feature-adoption metrics, query the existing source tables and do not create summary tables unless the user explicitly changes this requirement.

- Group by the natural day of `created_at` in the server/database timezone.
- Count `COUNT(DISTINCT uid)` per feature per day so one UID contributes at most once, regardless of how many rows it creates.
- Keep historical creation metrics stable by including source rows that were soft-deleted later.
- Weight: `weight_records`.
- User-created training tags: `training_tags` with `uid > 0` so system tags are excluded.
- Exercise action sets: `exercise_set_records`.
- Body and diet photo indexes: `body_photo_records`.
- Merge feature days in the service, sort date descending, then paginate. Return zero for a feature with no users on an otherwise active date.

## Workflow

1. Inspect both the Vue caller and the backing Go/local service before editing.
2. Preserve API response fields and existing filters while changing ordering.
3. Keep source-of-truth ordering in the backend/local service.
4. Add focused ordering and empty-list tests.
5. Run `npm run test:local`, `npm run build`, `gofmt` for changed Go files, `go test ./...`, and `git diff --check` as applicable.
