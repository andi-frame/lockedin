# Graph Report - lockedin  (2026-10-07)

## Corpus Check
- 105 files · ~216,404 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 13 file(s) not represented in the graph (top: (none) 7, .example 4, .lock 1)

## Summary
- 939 nodes · 2617 edges · 47 communities (35 shown, 12 thin omitted)
- Extraction: 95% EXTRACTED · 5% INFERRED · 0% AMBIGUOUS · INFERRED: 140 edges (avg confidence: 0.85)
- Token cost: 182,088 input · 0 output

## Community Hubs (Navigation)
- Service Test Fixtures
- Bun Task Scripts
- Auth and Domain Files
- Ledger Keys and Terms
- Auth Middleware and Tests
- Database Schema
- Root Package Manifest
- Binary Entrypoints
- Pact Membership Queries
- Check-in State Machine
- Ledger and Outbox Queries
- Store and Service Wiring
- Attachment and Decision Queries
- Calendar Date Type
- Check-in Read Queries
- Invite and Member Rows
- Project Invariants
- Ledger and Pact Spec
- API Stack and Edge
- Proof and Pact Creation
- Settlement Sweep Queries
- Agent Manual and Design
- Compose Infra and Tooling
- Scripts TS Config
- Backend Layering and Run Modes
- Phase 1 Plan Tasks
- Upload Pipeline Spec
- User Queries
- Append-only Ledger
- Review Power Model
- Passbook Visual Direction
- Contract-first Codegen
- Diagrams and Upload Hardening
- Job Queue Design
- Check-in Batch Insert
- Clock and Testing Strategy
- Attachment Counts
- CopyFrom Iterator
- Passbook Page Query
- Todo Stub Script
- Go Module
- Tools Module

## God Nodes (most connected - your core abstractions)
1. `Querier` - 61 edges
2. `Terms` - 35 edges
3. `Pact` - 30 edges
4. `scripts` - 26 edges
5. `newFixture()` - 23 edges
6. `Queries` - 21 edges
7. `PLAN.md implementation plan` - 21 edges
8. `transition` - 20 edges
9. `Transition()` - 19 edges
10. `step()` - 18 edges

## Surprising Connections (you probably didn't know these)
- `Append-only ledger invariant (balance = SUM(ledger_entries.amount))` --semantically_similar_to--> `Buku Tabungan (passbook visual world)`  [INFERRED] [semantically similar]
  AGENTS.md → .impeccable/surfaces/apps-web-src-app-app.md
- `Task 2.2 Fiber app and middleware (idempotency, rate limit)` --semantically_similar_to--> `Idempotency key per coin movement in same tx`  [INFERRED] [semantically similar]
  docs/PLAN.md → AGENTS.md
- `Own-world palette (cover teal, stamp violet, highlighter yellow, member colours)` --conceptually_related_to--> `Backer final power (override, dispute resolution) logged`  [INFERRED]
  .impeccable/surfaces/apps-web-src-app-app.md → PRODUCT.md
- `First viewport (countdown header, Hari ini, Perlu ditinjau, mini passbook)` --implements--> `Product principles (ledger is truth, rules first, power visible, quick proof, deadline is interface)`  [INFERRED]
  .impeccable/surfaces/apps-web-src-app-app.md → PRODUCT.md
- `Task 1.4 Store layer (sqlc)` --implements--> `Append-only ledger invariant (balance = SUM(ledger_entries.amount))`  [INFERRED]
  docs/PLAN.md → AGENTS.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Ledger integrity: append-only, idempotent, int64, single transition path** — agents_append_only_ledger, agents_idempotency_key, agents_int64_coins, agents_single_transition_impl, docs_plan_task_1_6_settlement [INFERRED 0.85]
- **Compose infra profile services** — deploy_compose_postgres, deploy_compose_redis, deploy_compose_garage, deploy_compose_mailpit [EXTRACTED 1.00]
- **Buku Tabungan UI direction realised across PLAN UI tasks** — _impeccable_surfaces_apps_web_src_app_app_buku_tabungan, _impeccable_surfaces_apps_web_src_app_app_own_world_palette, _impeccable_surfaces_apps_web_src_app_app_passbook_print, docs_plan_task_5_2_tokens, docs_plan_task_6_2_today, docs_plan_task_6_4_passbook [INFERRED 0.85]
- **Ledger Integrity Mechanisms** — docs_spec_invariant_l1, docs_spec_invariant_l2, docs_spec_invariant_l3, docs_spec_penalty_clamp, docs_architecture_forbid_mutation_trigger, docs_architecture_ledger_entries_table, docs_adr_0002_append_only_ledger_adr_0002 [INFERRED 0.95]
- **Review Power Checks and Balances** — docs_spec_backer_power, docs_spec_override, docs_spec_dispute, docs_architecture_decisions_table, docs_adr_0007_review_power_adr_0007 [INFERRED 0.95]
- **Presigned Upload and Media Processing Flow** — docs_architecture_upload_pipeline, docs_architecture_presigned_put, docs_architecture_garage, docs_architecture_attachments_table, docs_architecture_worker_service, docs_architecture_asynq_queues, docs_spec_attachment_limits [EXTRACTED 1.00]

## Communities (47 total, 12 thin omitted)

### Community 0 - "Service Test Fixtures"
Cohesion: 0.06
Nodes (70): newAuth(), TestLoginRateLimit(), TestRegisterLoginLogout(), FakeClock, NewFakeClock(), mustTime(), TestReviewDeadlineCountsFromCutoffForEarlySubmitters(), TestWindowDeadlines() (+62 more)

### Community 1 - "Bun Task Scripts"
Cohesion: 0.05
Nodes (61): ADR-0005, ADR-0008, @aws-sdk/client-s3, check, server, tool(), tools, [action, ...rest] (+53 more)

### Community 2 - "Auth and Domain Files"
Cohesion: 0.12
Nodes (16): CodeOf(), TestCodeOf(), ParseProofDoc(), safeLink(), TestParseProofDocDerivesTextAndWords(), TestParseProofDocRejects(), TestValidateLinks(), ValidateLinks() (+8 more)

### Community 3 - "Ledger Keys and Terms"
Cohesion: 0.07
Nodes (31): InitialPotKey(), PayoutKey(), PenaltyKey(), ReversalKey(), TestIdempotencyKeys(), Terms, hours(), countAttachments() (+23 more)

### Community 4 - "Auth Middleware and Tests"
Cohesion: 0.08
Nodes (21): TestRequireUserMiddleware(), TestCSRFIsBoundToTheSession(), TestNormaliseEmail(), TestPasswordHashAndVerify(), TestPasswordPolicyAndBadHashes(), NewLimiter(), ClearSessionCookies(), RequireUser() (+13 more)

### Community 5 - "Database Schema"
Cohesion: 0.10
Nodes (38): attachments, attachments_gc_idx, attachments_owner_id_idx, attachments_pact_id_idx, attachments_proof_id_idx, check_ins, check_ins_member_id_idx, check_ins_reviewer_queue_idx (+30 more)

### Community 6 - "Root Package Manifest"
Cohesion: 0.05
Nodes (38): devDependencies, @aws-sdk/client-s3, @types/bun, typescript, engines, bun, name, private (+30 more)

### Community 7 - "Binary Entrypoints"
Cohesion: 0.11
Nodes (18): main(), main(), Main(), envMap(), Config, Load(), LoadFrom(), oneOf() (+10 more)

### Community 8 - "Pact Membership Queries"
Cohesion: 0.10
Nodes (7): Queries, AcceptTermsParams, AddPactMemberParams, CountAcceptancesParams, GetPactForMemberParams, IncrementRestDaysUsedParams, SetPactStatusParams

### Community 9 - "Check-in State Machine"
Cohesion: 0.20
Nodes (16): CheckIn, Effect, Event, overrideAllowed(), ptr(), tick(), Transition(), validReason() (+8 more)

### Community 10 - "Ledger and Outbox Queries"
Cohesion: 0.07
Nodes (9): Queries, CreatePayoutParams, InsertLedgerEntryParams, InsertNotificationParams, InsertOutboxParams, LedgerEntry, ListNotificationsParams, MarkNotificationsReadParams (+1 more)

### Community 11 - "Store and Service Wiring"
Cohesion: 0.09
Nodes (13): Clock, Service, New(), New(), Connect(), Store, NewStore(), migrate() (+5 more)

### Community 12 - "Attachment and Decision Queries"
Cohesion: 0.12
Nodes (8): Queries, Attachment, AttachToProofParams, CreateAttachmentParams, InsertDecisionParams, ListDecisionsRow, ListReviewQueueRow, Payout

### Community 13 - "Calendar Date Type"
Cohesion: 0.11
Nodes (8): DateIn(), MustDate(), NewDate(), ParseDate(), TestCheckInDeadlinesAcrossIndonesianTimezones(), TestDateHelpers(), TestDateConversions(), Date

### Community 14 - "Check-in Read Queries"
Cohesion: 0.11
Nodes (3): GetCheckInForMemberParams, ListCheckInsForPactParams, Querier

### Community 15 - "Invite and Member Rows"
Cohesion: 0.12
Nodes (9): PactInvite, CreateInviteParams, Decision, GetPactMemberParams, ListOpenCheckInsForMemberParams, ListOpenCheckInsForMemberRow, ListPactMembersRow, ListSettlablePactsParams (+1 more)

### Community 16 - "Project Invariants"
Cohesion: 0.12
Nodes (16): UTC timestamptz, local_date, injected Clock, Money is int64 coins, no floats, Membership filter, non-member gets 404, No self-review; backer power logged as decisions row, Task 6.5 Review queue and check-in detail, Auto-approval after review window, Backer (penyokong), Backer final power (override, dispute resolution) logged (+8 more)

### Community 17 - "Ledger and Pact Spec"
Cohesion: 0.18
Nodes (17): ADR-0001 Coins are an IOU Ledger, pacts table, payouts table, Backer, Check-in Deadline Definitions, Doer, Tepati Functional Specification, Invariant L3: unique idempotency_key (+9 more)

### Community 18 - "API Stack and Edge"
Cohesion: 0.23
Nodes (16): ADR-0003 Go + Fiber, Next.js on Bun, spec-first OpenAPI, std-http strict server via Fiber adaptor, API Service (Go + Fiber), Caddy Edge Proxy, CSRF Double-Submit Token, Garage S3 Object Storage, Idempotency-Key Middleware, Fiber Middleware Stack (+8 more)

### Community 19 - "Proof and Pact Creation"
Cohesion: 0.14
Nodes (7): ProofInput, CreatePactParams, InsertProofParams, Notification, Outbox, Proof, UpdatePactTermsParams

### Community 20 - "Settlement Sweep Queries"
Cohesion: 0.17
Nodes (3): Queries, ListDueCheckInIDsParams, UpdateCheckInStateParams

### Community 21 - "Agent Manual and Design"
Cohesion: 0.20
Nodes (11): Buku Tabungan (passbook visual world), AGENTS.md operating manual, Append-only ledger invariant (balance = SUM(ledger_entries.amount)), Definition of done (every PR), One PLAN task per branch/PR workflow, Design workflow and direction (docs/design/README.md), Design quality loop (screenshots, impeccable detect, finish review, documenter), Wise DESIGN.md reference (money UI craft only) (+3 more)

### Community 22 - "Compose Infra and Tooling"
Cohesion: 0.21
Nodes (11): deploy/compose.yaml base compose, garage v2.4.1 service, mailpit service, Phase 0: Repository foundation (done), Task 4.1 BlobStore drivers (s3, fs), Task 7.2 compose.dev/prod and Caddy, Root package.json Bun scripts (infra:up, codegen, dev:*, test, lint, deploy:*), garage:init bootstrap script (scripts/garage-init.ts) (+3 more)

### Community 23 - "Scripts TS Config"
Cohesion: 0.15
Nodes (12): compilerOptions, allowImportingTsExtensions, module, moduleResolution, noEmit, noUncheckedIndexedAccess, skipLibCheck, strict (+4 more)

### Community 24 - "Backend Layering and Run Modes"
Cohesion: 0.21
Nodes (11): ADR-0006 PostgreSQL with sqlc and goose, ADR-0008 Three Run Modes via Bun Scripts, Compose profiles (infra/app/edge/tools/backup), scripts/dev.ts orchestrator, BlobStore Interface (s3 and fs drivers), domain package (pure, no I/O), RFC 9457 problem+json Error Codes, service package (use-cases, tx boundaries) (+3 more)

### Community 25 - "Phase 1 Plan Tasks"
Cohesion: 0.29
Nodes (11): Idempotency key per coin movement in same tx, Single transition implementation in internal/service, pure rules in internal/domain, redis:8 service (AOF), PLAN.md implementation plan, Task 1.3 Domain: terms, deadlines, check-in FSM, ledger math, Task 1.5 Services: pact lifecycle, Task 1.6 Check-in transitions and settlement, Task 1.7 Auth (argon2id, Redis sessions, CSRF, rate limit) (+3 more)

### Community 26 - "Upload Pipeline Spec"
Cohesion: 0.24
Nodes (10): ADR-0005 Garage Storage, Presigned Uploads, Server Compression, attachments table, check_ins table, Presigned PUT Upload, proofs table, Tiptap 3 Rich Text Editor, Upload and Compression Pipeline, Attachment Kinds and Limits (+2 more)

### Community 27 - "User Queries"
Cohesion: 0.31
Nodes (3): User, Queries, CreateUserParams

### Community 28 - "Append-only Ledger"
Cohesion: 0.33
Nodes (9): ADR-0002 Append-only Ledger with Idempotency Keys, ADR-0009 Visual Direction: Buku Tabungan, Passbook Print signature move, ADR Index and Supersede Process, forbid_mutation append-only trigger, ledger_entries table, Redis Aggregate Cache (pot balance, Today), Invariant L1: balance = SUM(ledger) (+1 more)

### Community 29 - "Review Power Model"
Cohesion: 0.36
Nodes (8): ADR-0007 Review Power Model, Stamp violet for human decisions, decisions table, Check-in State Machine, Dispute, Late Edits and Resubmission, Override, Rest Day

### Community 30 - "Passbook Visual Direction"
Cohesion: 0.29
Nodes (7): Surface brief: authenticated app shell, First viewport (countdown header, Hari ini, Perlu ditinjau, mini passbook), Own-world palette (cover teal, stamp violet, highlighter yellow, member colours), Passbook print signature move, Task 5.2 Design tokens and primitives (Amount, Countdown, StatusChip, MemberLine), Task 6.2 Today screen, Task 6.4 Pact page: passbook and calendar

### Community 31 - "Contract-first Codegen"
Cohesion: 0.33
Nodes (6): Contract-first (openapi.yaml, goose migration, sqlc, codegen), sqlc.yaml config, internal/store generated package (pgx/v5, uuid/time overrides), postgres:18 service, Task 1.4 Store layer (sqlc), Task 2.1 OpenAPI contract (next unchecked)

### Community 32 - "Diagrams and Upload Hardening"
Cohesion: 0.33
Nodes (6): Upload hardening (presigned, sniffed type, EXIF strip, server limits), Diagrams README (archify), tepati-architecture.html, checkin-lifecycle.html, upload-sequence.html, Task 4.2 Upload intent, complete, media processing

### Community 33 - "Job Queue Design"
Cohesion: 0.40
Nodes (4): ADR-0004 asynq on Redis, River (Postgres queue) fallback, asynq Priority Queues (critical/default/media), Media Queue Backpressure (MEDIA_QUEUE_MAX)

### Community 35 - "Clock and Testing Strategy"
Cohesion: 0.50
Nodes (4): Injected Clock Interface, Test Clock Endpoint (/_test/clock), Testing Strategy, Non-functional Requirements

## Knowledge Gaps
- **96 isolated node(s):** `github.com/andi-frame/lockedin/apps/server`, `Queries`, `github.com/andi-frame/lockedin/apps/server/tools`, `name`, `private` (+91 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 157 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **12 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `fixture` connect `Service Test Fixtures` to `Store and Service Wiring`, `Pact Membership Queries`, `Auth and Domain Files`, `User Queries`?**
  _High betweenness centrality (0.023) - this node is a cross-community bridge._
- **What connects `github.com/andi-frame/lockedin/apps/server`, `Queries`, `github.com/andi-frame/lockedin/apps/server/tools` to the rest of the system?**
  _96 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Service Test Fixtures` be split into smaller, more focused modules?**
  _Cohesion score 0.05929989550679206 - nodes in this community are weakly interconnected._
- **Why does `Terms` connect `Ledger Keys and Terms` to `Service Test Fixtures`, `Auth and Domain Files`, `Check-in State Machine`, `Attachment and Decision Queries`, `Calendar Date Type`?**
  _High betweenness centrality (0.023) - this node is a cross-community bridge._
- **Should `Bun Task Scripts` be split into smaller, more focused modules?**
  _Cohesion score 0.05280437756497948 - nodes in this community are weakly interconnected._
- **Why does `Date` connect `Calendar Date Type` to `Auth and Domain Files`, `Ledger Keys and Terms`, `Invite and Member Rows`?**
  _High betweenness centrality (0.018) - this node is a cross-community bridge._
- **Should `Auth and Domain Files` be split into smaller, more focused modules?**
  _Cohesion score 0.1189873417721519 - nodes in this community are weakly interconnected._