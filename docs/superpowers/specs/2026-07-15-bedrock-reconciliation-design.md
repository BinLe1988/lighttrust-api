# Bedrock Reconciliation Plugin Design

Date: 2026-07-15

## 1. Objective

Add an in-process, provider-oriented reconciliation subsystem to new-api. Amazon Bedrock is the first provider. The subsystem must reconcile all three accounting levels without mutating historical consumption or quota data in its first release:

1. Request-level usage: correlate an internal request with one Bedrock invocation and compare model and token usage.
2. Daily aggregate usage and cost: compare internal aggregates, Bedrock invocation logs, and AWS Cost and Usage Report (CUR 2.0) amounts.
3. Account-level spend: explain AWS Bedrock net billed cost using attributable channel cost plus credits, refunds, tax, and other adjustments.

The design must remain extensible to providers other than Bedrock while fitting the existing Router -> Controller -> Service -> Model architecture.

## 2. Scope

### Included

- A provider-neutral reconciliation interface and normalized records.
- Amazon Bedrock data ingestion through an independently configured IAM role.
- Bedrock model invocation log ingestion from CloudWatch Logs or S3.
- CUR 2.0 retrieval through S3/Athena for invoice-accurate aggregate cost.
- Cost Explorer retrieval for account-level provisional checks.
- CloudWatch metric retrieval as a completeness signal.
- Request, daily, and account-level reconciliation results.
- Scheduled and manual reconciliation runs with persistent cursors and idempotency.
- Root/admin APIs, permission checks, audit logs, dashboard pages, filtering, and CSV export.
- Explicit data maturity and match-confidence states.

### Excluded from the first release

- Automatic quota adjustments or refunds.
- Mutation of historical consume logs.
- Automatic channel disablement.
- Prompt or model response archival.
- Reconciliation for `bedrock-mantle` endpoints.
- Provider implementations other than Bedrock.

## 3. Architectural Decision

Use a modular in-process plugin rather than an external service or data-warehouse-only workflow.

The module integrates with existing channels, logs, authorization, system tasks, and the default frontend. Its provider boundary and normalized persistence model must be independent enough to extract into a service later if volume requires it.

Proposed package responsibilities:

- `reconcile/`: provider interfaces, normalized types, statuses, and validation.
- `reconcile/bedrock/`: STS credentials, invocation logs, CUR/Athena, Cost Explorer, and CloudWatch adapters.
- `service/reconcile_*.go`: ingestion orchestration, matching, aggregation, and run lifecycle.
- `model/reconcile_*.go`: configuration, cursor, normalized data, result, and summary persistence.
- `controller/reconcile.go`: configuration, diagnostics, run, query, retry, and export handlers.
- `router/`: permission-protected reconciliation routes.
- `web/default/src/features/reconciliation/`: reconciliation dashboard and configuration UI.

## 4. Provider Contract

The provider contract expresses upstream accounting capabilities rather than AWS-specific APIs:

```go
type Provider interface {
    PullInvocations(ctx context.Context, cursor Cursor) ([]Invocation, Cursor, error)
    PullDailyCosts(ctx context.Context, period Period) ([]CostBucket, error)
    PullAccountAdjustments(ctx context.Context, period Period) ([]AccountAdjustment, error)
    CheckAccess(ctx context.Context) AccessDiagnostic
}
```

Implementations may report unsupported capabilities explicitly. Pull operations must be paginated, restartable, and safe to repeat.

## 5. AWS Access and Configuration

Each Bedrock reconciliation configuration contains:

- A stable configuration ID and display name.
- AWS account ID.
- IAM Role ARN.
- Required External ID.
- Enabled Regions.
- Channel mappings for the account and Regions.
- Invocation logging source: CloudWatch log group or S3 bucket/prefix.
- CUR 2.0 S3 location, Glue catalog/database/table, and Athena workgroup/output location.
- Cost Explorer enablement.
- Scheduling, lookback, maturity delay, tolerance, and alert thresholds.

The application uses STS `AssumeRole`. Temporary credentials live only in memory and are refreshed before expiry. Role session names contain an instance identifier and reconciliation run ID. No AWS secret or session token is persisted or returned by an API.

IAM access must be restricted to the configured log group, S3 prefixes, Glue resources, Athena workgroup, Cost Explorer reads, and CloudWatch metrics. Configuration diagnostics check every capability separately so a missing optional permission does not hide a working data source.

## 6. Bedrock Request Correlation

All Bedrock runtime calls made by this gateway must attach request metadata:

```json
{
  "lighttrust_request_id": "<internal request ID>",
  "channel_id": "<channel ID>"
}
```

For `InvokeModel` and `InvokeModelWithResponseStream`, this is sent as the SigV4-signed `X-Amzn-Bedrock-Request-Metadata` header. Metadata values must contain no username, token key, prompt, response, credential, or other personal/sensitive data.

The AWS SDK response request ID must also be captured and stored in the existing `Log.UpstreamRequestId` field for synchronous and streaming responses. Error responses should capture a request ID when AWS exposes one.

Matching order is:

1. `requestMetadata.lighttrust_request_id` to `Log.RequestId`.
2. Bedrock invocation `requestId` to `Log.UpstreamRequestId`.
3. Channel, model, operation, bounded timestamp window, and token signature.

The third method produces only a `probable` result. It must never drive an automatic financial adjustment. A non-unique result remains `ambiguous`; the engine must not guess.

## 7. Persistence Model

Reconciliation tables are separate from existing consume logs.

### `reconcile_configs`

Provider, AWS identifiers, role and data-source settings, channel mappings, schedule, thresholds, enabled state, and timestamps. Sensitive configuration fields must be omitted from ordinary API responses.

### `reconcile_runs`

Run ID, configuration, data source, requested period, effective period, maturity, status, cursor, query execution IDs, counters, structured error, runner identity, and timestamps.

### `upstream_invocations`

Provider, account, Region, request ID, local request metadata, timestamp, operation, model/inference profile, input/output/cache-read/cache-write tokens, identity ARN, source location, ingestion run, and source hash.

The unique key is provider + account + Region + upstream request ID. Request and response bodies are never persisted.

### `upstream_cost_buckets`

Provider, account, billing period, Region, model/pricing resource, operation, usage type, token category, service tier, routing type, usage quantity, unblended cost, net cost when available, currency, CUR identity fields, maturity, ingestion run, and source hash.

### `reconcile_items`

Configuration, internal log identity, invocation identity, match method, confidence, status, internal/upstream model and token values, computed deltas, allocated request cost, maturity, first/last observed times, and resolution metadata.

Statuses include `matched`, `token_mismatch`, `model_mismatch`, `internal_missing`, `upstream_missing`, `duplicate`, `ambiguous`, and `pending`.

### `reconcile_daily_summaries`

Date plus account, Region, channel, model, operation, service tier, routing type, and token category dimensions. Measures include request counts, all token categories, internal estimated cost, invocation-log estimated cost, CUR cost, absolute delta, percentage delta, unmatched counts, and maturity.

### `reconcile_account_summaries`

Billing period, AWS gross Bedrock cost, credits, refunds, tax/other applicable adjustments, AWS net cost, cost attributed to configured channels, unattributed cost, and unexplained delta.

Amounts are calculated with `shopspring/decimal`, never binary floating point. Database columns and migrations must preserve the necessary precision on SQLite, MySQL, and PostgreSQL. Currency is always explicit.

## 8. Reconciliation Semantics

### Request level

Compare internal and invocation-log model identity and all available token categories. Cache read and cache write tokens are first-class values. Request-level cost is labeled either `estimated` from a maintained Bedrock rate card or `allocated` from a CUR cost bucket. It is never labeled invoice-exact because CUR does not include a per-request identifier.

### Daily level

Aggregate by date, account, Region, channel, Bedrock model, operation, service tier, routing type, and token category. Compare:

- Successful request count.
- Input and output tokens.
- Cache read and cache write tokens.
- Internally estimated upstream cost.
- Invocation-log estimated cost.
- CUR billed cost.

CUR is the financial source of truth. Invocation logs explain request composition. CloudWatch invocation and token metrics are completeness checks only.

### Account level

For postpaid AWS accounts, account reconciliation is spend reconciliation rather than a provider balance:

```text
Bedrock gross cost - credits - refunds + applicable tax/adjustments = AWS net cost
AWS net cost - cost attributed to configured channels = unexplained account difference
```

The UI must distinguish attributable channel cost, unconfigured Bedrock use, billing adjustments, and unexplained difference.

## 9. Scheduling, Idempotency, and Maturity

Use the existing `SystemTask` runner and database lease mechanism. Separate scheduled handlers ingest invocation logs, ingest CUR, fetch provisional account cost, reconcile requests, build daily summaries, and build account summaries.

Every ingestion operation maintains a persistent cursor. Writes use natural unique keys and source hashes, making a repeated page or full rerun idempotent.

Default maturity behavior:

- Invocation records remain `pending` until the configurable delay passes; default 30 minutes.
- CUR periods remain `provisional` while AWS may update them; rerun the most recent three days by default.
- Closed billing periods become `final` only after the configured close delay.
- A missing record is not reported as a discrepancy until its source-specific maturity window has passed.

## 10. Error Handling

- STS, CloudWatch, S3, Athena, CUR, and Cost Explorer failures are recorded independently.
- Access-denied and invalid-configuration errors stop that source immediately and do not retry indefinitely.
- Throttling and transient AWS failures use bounded exponential backoff with jitter.
- Athena query execution IDs are persisted, allowing polling to resume after process restart.
- A malformed source record is stored as a rejected-record diagnostic and does not fail the whole batch.
- Runs record scanned, inserted, updated, duplicate, rejected, matched, mismatched, and failed counters.
- A run cannot promote summaries to final when a required source failed or is immature.

## 11. Authorization, Audit, and Security

- Configuration and manual execution require root or dedicated Casbin permissions.
- Read-only reconciliation views may be granted separately to administrators.
- Configuration changes, manual runs, retries, exports, and any future resolution action create management audit logs.
- API responses and backend logs must redact role external IDs, temporary credentials, signed URLs, and sensitive AWS errors.
- Reconciliation storage contains no prompt or response body.
- Ordinary users cannot access upstream account, channel, identity ARN, cost, or mismatch data.

## 12. API and User Interface

The backend exposes configuration CRUD, access diagnostics, manual run, retry, run history, request results, daily summaries, account summaries, and CSV export endpoints.

The default frontend adds:

- Overview: latest runs, data maturity, mismatch counts, and three-level cost deltas.
- Request reconciliation: filtering by date, channel, model, status, local request ID, and upstream request ID.
- Daily summary: internal, invocation-log, and CUR token/cost comparisons.
- Account summary: gross cost, credits/refunds, net cost, attributed cost, and unexplained difference.
- Run history: source, period, cursor, statistics, errors, and retry action.
- Configuration: role, Regions, channel mappings, sources, schedule, tolerance, and access diagnostics.

CSV exports are asynchronous when the result exceeds a bounded synchronous limit.

## 13. Testing and Acceptance Criteria

Tests must cover observable reconciliation and financial invariants:

- Bedrock request metadata injection for streaming and non-streaming SDK calls.
- AWS SDK request ID capture on success and AWS errors where available.
- Deterministic parsing of invocation log, CUR, and Cost Explorer fixtures.
- Exact, probable, duplicate, missing, ambiguous, and mismatch matching cases.
- Cache read/write token categorization and pricing.
- Cross-Region inference profile and model normalization.
- CUR backfill transitions from provisional to final.
- Repeated ingestion and reruns produce no duplicate records or totals.
- Decimal aggregation has no floating-point drift.
- SQLite, MySQL, and PostgreSQL migrations and query behavior remain compatible.
- System task lease prevents duplicate active runs across nodes.
- Interrupted Athena runs resume from persisted execution state.
- Permission diagnostics distinguish each missing AWS permission.
- Frontend authorization, filtering, pagination, maturity display, and export behavior.

Ordinary tests use AWS SDK mocks and fixed source fixtures; they do not access a real AWS account.

The first release is accepted when a configured Bedrock account can ingest all required sources, correlate gateway invocations, produce mature daily and account summaries, expose unresolved differences without modifying financial records, and repeat the same period idempotently.

## 14. Rollout

1. Add schema, provider contract, and read-only configuration diagnostics.
2. Capture Bedrock request metadata and upstream request IDs.
3. Ingest invocation logs and release request-level reconciliation behind a disabled-by-default feature flag.
4. Add CUR/Athena ingestion and daily reconciliation.
5. Add Cost Explorer/account summaries and CloudWatch completeness checks.
6. Add dashboard pages, export, alert thresholds, and operational documentation.
7. Enable schedules only after a successful manual backfill and permission diagnostic.

Automatic adjustment remains a separately designed and approved future feature.
