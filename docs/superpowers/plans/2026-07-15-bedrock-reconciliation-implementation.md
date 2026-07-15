# Bedrock Reconciliation Plugin Implementation Plan

Design: `docs/superpowers/specs/2026-07-15-bedrock-reconciliation-design.md`

## Delivery strategy

Build the feature in independently testable vertical increments. Keep reconciliation disabled by default until AWS access diagnostics and a manual backfill succeed. Every schema and query must support SQLite, MySQL, and PostgreSQL.

## Phase 1: Domain foundation and persistence

1. Add `reconcile` package:
   - Provider interface and capability reporting.
   - Cursor, period, invocation, cost bucket, adjustment, diagnostic, maturity, match method, confidence, and status types.
   - Validation and normalization helpers with deterministic tests.
2. Add model files for:
   - Configurations and channel mappings.
   - Runs and cursors.
   - Upstream invocations and rejected records.
   - Cost buckets.
   - Request items, daily summaries, and account summaries.
3. Use decimal-backed amount types and explicit currency.
4. Add natural unique indexes for ingestion idempotency.
5. Register tables in the existing migration flow.
6. Add SQLite migration and idempotency tests; exercise shared query construction used by all dialects.

Exit criteria: models migrate, normalized fixtures round-trip, duplicate invocation/cost ingestion updates rather than duplicates, and no existing tests regress.

## Phase 2: Bedrock request correlation

1. Add a small request-correlation helper in `relay/channel/aws`.
2. Inject `lighttrust_request_id` and `channel_id` into `X-Amzn-Bedrock-Request-Metadata` for SDK `InvokeModel` and `InvokeModelWithResponseStream` calls.
3. Ensure the metadata header is included through AWS SDK middleware so SigV4 signs it.
4. Extract AWS SDK response Request ID for normal, streaming, Nova, and error paths.
5. Store the value in `common.UpstreamRequestIdKey` before consume/error logging.
6. Add deterministic SDK middleware tests for metadata and request ID capture.

Exit criteria: every existing Bedrock runtime path emits local correlation metadata and stores AWS Request ID when AWS returns one.

## Phase 3: AWS role and access diagnostics

1. Add AWS SDK dependencies only for services actually used: STS, CloudWatch Logs, S3, Athena, Glue metadata if necessary, Cost Explorer, and CloudWatch.
2. Implement per-config STS AssumeRole credential provider with required External ID and bounded in-memory caching.
3. Implement source-specific access diagnostics.
4. Redact credentials, External ID, signed locations, and sensitive AWS error details.
5. Add controller/model CRUD for root users and dedicated Casbin permissions for read/configure/run/export.
6. Audit configuration changes and diagnostic runs.

Exit criteria: an administrator can save a redacted configuration and diagnose each AWS capability independently without running reconciliation.

## Phase 4: Invocation log ingestion

1. Implement CloudWatch Logs Insights reader with persisted query ID and pagination/resume.
2. Implement S3 invocation log reader for gzipped batched JSON records.
3. Parse schema-versioned Bedrock invocation records without storing bodies.
4. Normalize foundation model and inference profile identifiers while preserving originals.
5. Upsert by provider/account/Region/request ID and persist source hash.
6. Store malformed records as bounded rejected diagnostics.
7. Add ingestion run counters and persistent cursor/lookback behavior.

Exit criteria: a repeated fixture/backfill is idempotent and produces complete normalized invocation rows including cache token categories.

## Phase 5: Request reconciliation

1. Query mature internal consume/error logs for mapped Bedrock channels.
2. Match by local request metadata, then upstream request ID, then bounded signature.
3. Never auto-select a non-unique probable match.
4. Produce all request statuses and field deltas.
5. Re-evaluate pending/missing items during configurable lookback.
6. Add request-result API with indexed filtering and stable pagination.
7. Add exact fixture tests for each match/status transition.

Exit criteria: request-level results explain matched, mismatched, duplicate, ambiguous, and missing calls without modifying source logs.

## Phase 6: CUR/Athena and daily cost

1. Validate configured Glue/Athena/CUR resources.
2. Submit bounded, parameterized Athena queries for Bedrock CUR rows.
3. Persist and resume Athena execution IDs.
4. Parse model, Region, operation, service tier, cross-Region routing, and all token types from CUR dimensions.
5. Upsert cost buckets with source hashes and provisional/final maturity.
6. Aggregate internal logs, invocation logs, and CUR into daily summaries.
7. Allocate CUR bucket cost to request rows only when required, labeling it `allocated`.
8. Add daily API, CSV export, and decimal precision tests.

Exit criteria: a daily summary reconciles request count, all token categories, estimated cost, and invoice cost, with explicit maturity.

## Phase 7: Account reconciliation and completeness

1. Fetch Cost Explorer daily Bedrock cost for provisional account checks.
2. Parse credits, refunds, and applicable adjustments from billing data available to the configured account.
3. Fetch CloudWatch Bedrock invocation/token metrics as completeness evidence.
4. Build account summaries that distinguish configured-channel cost, unattributed Bedrock use, adjustments, and unexplained difference.
5. Prevent final maturity if a required data source failed or remains provisional.
6. Add threshold evaluation and backend warning events; do not mutate financial state.

Exit criteria: account summaries satisfy the documented cost identity and expose source/maturity gaps.

## Phase 8: Scheduling and operations

1. Register separate scheduled SystemTask handlers for each ingestion and reconciliation stage.
2. Chain stages by persisted readiness rather than in-memory callbacks.
3. Use existing database leases for multi-node deduplication.
4. Add manual period run, safe retry, and run-history endpoints.
5. Add retention controls for rejected diagnostics and operational run details.
6. Document initial AWS setup, IAM policy boundaries, logging, CUR, and Athena prerequisites.

Exit criteria: scheduled and manual runs recover after restart and cannot create two active runs for the same config/source/period.

## Phase 9: Default frontend

1. Add typed API client and query-key factory.
2. Add protected routes and navigation permission checks.
3. Implement overview, request results, daily summary, account summary, run history, and configuration/diagnostic screens.
4. Add server-side filtering, pagination, maturity badges, and accessible difference presentation.
5. Add bounded synchronous CSV export with asynchronous handling for large exports.
6. Add all user-visible strings through i18next and synchronize locales.

Exit criteria: authorized administrators can configure, run, inspect, filter, retry, and export reconciliation without seeing sensitive credentials.

## Verification gates

Run at each relevant phase:

- Focused package tests for changed backend packages.
- `go test ./...` before final handoff.
- Migration tests using SQLite plus dialect-safe SQL review.
- `bun run typecheck`, `bun run lint`, and `bun run build` in `web/default` after frontend work.
- `git diff --check` and a scope review that excludes pre-existing workspace changes.

## Commit strategy

Keep commits reviewable and dependency ordered:

1. Domain and schema.
2. Bedrock request correlation.
3. AWS role and invocation ingestion.
4. Request reconciliation.
5. CUR and daily summaries.
6. Account summaries and scheduling.
7. APIs and frontend.
8. Documentation and final verification fixes.
