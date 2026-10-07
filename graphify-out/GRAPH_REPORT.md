# Graph Report - lockedin  (2026-10-07)

## Corpus Check
- 129 files · ~270,932 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 13 file(s) not represented in the graph (top: (none) 7, .example 4, .lock 1)

## Summary
- 1950 nodes · 5732 edges · 151 communities (76 shown, 75 thin omitted)
- Extraction: 95% EXTRACTED · 5% INFERRED · 0% AMBIGUOUS · INFERRED: 265 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `bb6098f8`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- sampleTerms
- garage-init.ts
- api_integration_test.go
- CheckIn
- newStack
- 20261007000001_initial_schema.sql
- scripts
- Config
- Queries
- Transition
- Queries
- DBTX
- Queries
- Date
- context.Context
- time.Time
- Project invariants (10 rules)
- Tepati Functional Specification
- Tepati Architecture
- ServerInterface
- github.com/google/uuid.UUID
- AGENTS.md operating manual
- deploy/compose.yaml base compose
- compilerOptions
- service package (use-cases, tx boundaries)
- PLAN.md implementation plan
- Upload and Compression Pipeline
- CreateUserParams
- ADR-0002 Append-only Ledger with Idempotency Keys
- Check-in State Machine
- Direction contract (THESIS/OWN-WORLD/STORY/FIRST VIEWPORT/SIGNATURE MOVE/FORM/FINISH)
- Contract-first (openapi.yaml, goose migration, sqlc, codegen)
- Diagrams README (archify)
- server_test.go
- StrictServerInterface
- Injected Clock Interface
- testing.T
- iteratorForInsertCheckIns
- MemberView
- todo.ts
- github.com/andi-frame/lockedin/apps/server
- github.com/andi-frame/lockedin/apps/server/tools
- User
- wise.DESIGN.md
- Problem
- Pact
- CreateUploadRequestObject
- GetReviewQueueParams
- api.gen.go
- spec_test.go
- at
- Handlers
- fixture
- fiber.Ctx
- stub
- dev.ts
- .GetReviewQueue
- CheckInDetail
- Pact
- env.ts
- log/slog.Logger
- run
- db.ts
- me
- New
- setup.test.ts
- .JoinInvite
- ServerInterfaceWrapper
- strictHandler
- observe
- Tepati: project status and handoff
- github.com/oapi-codegen/runtime/types.UUID
- CheckIn
- redisStorage
- .MarkPayoutPaid
- LedgerLine
- .PreviewInvite
- ADR Index and Supersede Process
- codegen.ts
- .GetToday
- time.Duration
- idempotency
- apiTerms
- schema.d.ts
- RegisterHandlersWithOptions
- User
- Notification
- .AcceptPact
- .DisputeCheckIn
- ErrorCode
- .GetCheckIn
- .GetMe
- .GetToday
- .ListPacts
- .Login
- .Logout
- .MarkPayoutPaid
- .Register
- .ResolveDispute
- .ListPactCheckIns
- AcceptPact4XXApplicationProblemPlusJSONResponse
- CompleteUpload4XXApplicationProblemPlusJSONResponse
- CompleteUploaddefaultApplicationProblemPlusJSONResponse
- ConfirmPayout4XXApplicationProblemPlusJSONResponse
- ConfirmPayoutdefaultApplicationProblemPlusJSONResponse
- CreatePact4XXApplicationProblemPlusJSONResponse
- CreateUpload4XXApplicationProblemPlusJSONResponse
- CreateUploaddefaultApplicationProblemPlusJSONResponse
- DeclareRest200JSONResponse
- DeclareRest4XXApplicationProblemPlusJSONResponse
- DeclareRestdefaultApplicationProblemPlusJSONResponse
- DisputeCheckIn200JSONResponse
- GetAttachment4XXApplicationProblemPlusJSONResponse
- GetAttachmentdefaultApplicationProblemPlusJSONResponse
- GetCheckIn200JSONResponse
- GetCheckIn4XXApplicationProblemPlusJSONResponse
- GetCheckIndefaultApplicationProblemPlusJSONResponse
- GetMe4XXApplicationProblemPlusJSONResponse
- GetPactdefaultApplicationProblemPlusJSONResponse
- GetReviewQueuedefaultApplicationProblemPlusJSONResponse
- ListLedger4XXApplicationProblemPlusJSONResponse
- ListLedgerdefaultApplicationProblemPlusJSONResponse
- ListNotifications4XXApplicationProblemPlusJSONResponse
- ListPactCheckInsdefaultApplicationProblemPlusJSONResponse
- ListPacts4XXApplicationProblemPlusJSONResponse
- Login4XXApplicationProblemPlusJSONResponse
- Logout4XXApplicationProblemPlusJSONResponse
- MarkNotificationsRead4XXApplicationProblemPlusJSONResponse
- MarkNotificationsReaddefaultApplicationProblemPlusJSONResponse
- MarkPayoutPaid4XXApplicationProblemPlusJSONResponse
- OverrideCheckIn200JSONResponse
- OverrideCheckIndefaultApplicationProblemPlusJSONResponse
- PreviewInvite4XXApplicationProblemPlusJSONResponse
- ProposePact4XXApplicationProblemPlusJSONResponse
- ProposePactdefaultApplicationProblemPlusJSONResponse
- Register4XXApplicationProblemPlusJSONResponse
- ResolveDispute200JSONResponse
- ResolveDispute4XXApplicationProblemPlusJSONResponse
- SubmitProof200JSONResponse
- SubmitProof4XXApplicationProblemPlusJSONResponse
- SubmitProofdefaultApplicationProblemPlusJSONResponse
- UpdatePact4XXApplicationProblemPlusJSONResponse
- UpdatePactdefaultApplicationProblemPlusJSONResponse
- ValidReason

## God Nodes (most connected - your core abstractions)
1. `Problem` - 71 edges
2. `Querier` - 68 edges
3. `Terms` - 39 edges
4. `StrictServerInterface` - 37 edges
5. `newFixture()` - 37 edges
6. `ServerInterface` - 36 edges
7. `newEnv()` - 35 edges
8. `ServerInterfaceWrapper` - 34 edges
9. `strictHandler` - 34 edges
10. `me()` - 29 edges

## Surprising Connections (you probably didn't know these)
- `Append-only ledger invariant (balance = SUM(ledger_entries.amount))` --semantically_similar_to--> `Buku Tabungan (passbook visual world)`  [INFERRED] [semantically similar]
  AGENTS.md → .impeccable/surfaces/apps-web-src-app-app.md
- `Task 2.2 Fiber app and middleware (idempotency, rate limit)` --semantically_similar_to--> `Idempotency key per coin movement in same tx`  [INFERRED] [semantically similar]
  docs/PLAN.md → AGENTS.md
- `Task 6.5 Review queue and check-in detail` --implements--> `Backer final power (override, dispute resolution) logged`  [INFERRED]
  docs/PLAN.md → PRODUCT.md
- `Own-world palette (cover teal, stamp violet, highlighter yellow, member colours)` --conceptually_related_to--> `Backer final power (override, dispute resolution) logged`  [INFERRED]
  .impeccable/surfaces/apps-web-src-app-app.md → PRODUCT.md
- `Worked example: 30-day pact ending at 800 coins` --conceptually_related_to--> `MVP money model: coins as IOU ledger`  [INFERRED]
  docs/PLAN.md → PRODUCT.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Compose infra profile services** — deploy_compose_postgres, deploy_compose_redis, deploy_compose_garage, deploy_compose_mailpit [EXTRACTED 1.00]
- **Presigned Upload and Media Processing Flow** — docs_architecture_upload_pipeline, docs_architecture_presigned_put, docs_architecture_garage, docs_architecture_attachments_table, docs_architecture_worker_service, docs_architecture_asynq_queues, docs_spec_attachment_limits [EXTRACTED 1.00]
- **Ledger integrity: append-only, idempotent, int64, single transition path** — agents_append_only_ledger, agents_idempotency_key, agents_int64_coins, agents_single_transition_impl, docs_plan_task_1_6_settlement [INFERRED 0.85]
- **Buku Tabungan UI direction realised across PLAN UI tasks** — _impeccable_surfaces_apps_web_src_app_app_buku_tabungan, _impeccable_surfaces_apps_web_src_app_app_own_world_palette, _impeccable_surfaces_apps_web_src_app_app_passbook_print, docs_plan_task_5_2_tokens, docs_plan_task_6_2_today, docs_plan_task_6_4_passbook [INFERRED 0.85]
- **Ledger Integrity Mechanisms** — docs_spec_invariant_l1, docs_spec_invariant_l2, docs_spec_invariant_l3, docs_spec_penalty_clamp, docs_architecture_forbid_mutation_trigger, docs_architecture_ledger_entries_table, docs_adr_0002_append_only_ledger_adr_0002 [INFERRED 0.95]
- **Review Power Checks and Balances** — docs_spec_backer_power, docs_spec_override, docs_spec_dispute, docs_architecture_decisions_table, docs_adr_0007_review_power_adr_0007 [INFERRED 0.95]

## Communities (151 total, 75 thin omitted)

### Community 0 - "sampleTerms"
Cohesion: 0.10
Nodes (21): mustTime(), TestReviewDeadlineCountsFromCutoffForEarlySubmitters(), TestWindowDeadlines(), ClampBonus(), ClampPenalty(), PenaltyEntry(), TestClampBonus(), TestClampPenalty() (+13 more)

### Community 1 - "garage-init.ts"
Cohesion: 0.23
Nodes (15): ADR-0005, garage(), garageInit(), garageOk(), persistEnv(), [action = "up", ...rest], compose, composeBase() (+7 more)

### Community 2 - "api_integration_test.go"
Cohesion: 0.09
Nodes (24): TestPasswordHashAndVerify(), TestPasswordPolicyAndBadHashes(), checkPasswordPolicy(), HashPassword(), VerifyPassword(), ParseProofDoc(), safeLink(), TestParseProofDocDerivesTextAndWords() (+16 more)

### Community 3 - "CheckIn"
Cohesion: 0.12
Nodes (17): Event, InitialPotKey(), PayoutKey(), PenaltyKey(), ReversalKey(), TestIdempotencyKeys(), countAttachments(), Service (+9 more)

### Community 4 - "newStack"
Cohesion: 0.05
Nodes (49): TestCSRFIsBoundToTheSession(), ClearSessionCookies(), RequireUser(), safeMethod(), SetSessionCookies(), UserID(), Service, NewService() (+41 more)

### Community 5 - "20261007000001_initial_schema.sql"
Cohesion: 0.10
Nodes (38): attachments, attachments_gc_idx, attachments_owner_id_idx, attachments_pact_id_idx, attachments_proof_id_idx, check_ins, check_ins_member_id_idx, check_ins_reviewer_queue_idx (+30 more)

### Community 6 - "scripts"
Cohesion: 0.05
Nodes (40): devDependencies, @aws-sdk/client-s3, openapi-typescript, @types/bun, typescript, engines, bun, name (+32 more)

### Community 7 - "Config"
Cohesion: 0.21
Nodes (12): envMap(), Config, Load(), LoadFrom(), oneOf(), TestFSDriverDoesNotNeedS3Keys(), TestLoadAppliesDefaults(), TestLoadListsEveryMissingVariable() (+4 more)

### Community 8 - "Queries"
Cohesion: 0.06
Nodes (13): PactInvite, Queries, AcceptTermsParams, AddPactMemberParams, CountAcceptancesParams, CreateInviteParams, CreatePactParams, GetPactForMemberParams (+5 more)

### Community 9 - "Transition"
Cohesion: 0.12
Nodes (30): CheckIn, Effect, overrideAllowed(), ptr(), autoApproved(), ctx(), disputed(), kinds() (+22 more)

### Community 10 - "Queries"
Cohesion: 0.07
Nodes (13): Queries, Proof, ProofInput, CreatePayoutParams, InsertLedgerEntryParams, InsertNotificationParams, InsertOutboxParams, LedgerEntry (+5 more)

### Community 12 - "Queries"
Cohesion: 0.13
Nodes (7): Queries, AttachToProofParams, CountAttachmentsByStatusParams, CountAttachmentsByStatusRow, CreateAttachmentParams, InsertDecisionParams, InsertProofParams

### Community 13 - "Date"
Cohesion: 0.11
Nodes (8): DateIn(), MustDate(), NewDate(), ParseDate(), TestCheckInDeadlinesAcrossIndonesianTimezones(), TestDateHelpers(), TestDateConversions(), Date

### Community 15 - "time.Time"
Cohesion: 0.10
Nodes (11): Today, Terms, hours(), Queries, SystemClock, pactCursor, reviewCursor, Decision (+3 more)

### Community 16 - "Project invariants (10 rules)"
Cohesion: 0.12
Nodes (16): UTC timestamptz, local_date, injected Clock, Money is int64 coins, no floats, Membership filter, non-member gets 404, No self-review; backer power logged as decisions row, Task 6.5 Review queue and check-in detail, Auto-approval after review window, Backer (penyokong), Backer final power (override, dispute resolution) logged (+8 more)

### Community 17 - "Tepati Functional Specification"
Cohesion: 0.21
Nodes (14): ADR-0001 Coins are an IOU Ledger, pacts table, payouts table, Backer, Check-in Deadline Definitions, Doer, Tepati Functional Specification, Notification Triggers (+6 more)

### Community 18 - "Tepati Architecture"
Cohesion: 0.23
Nodes (16): ADR-0003 Go + Fiber, Next.js on Bun, spec-first OpenAPI, std-http strict server via Fiber adaptor, API Service (Go + Fiber), Caddy Edge Proxy, CSRF Double-Submit Token, Garage S3 Object Storage, Idempotency-Key Middleware, Fiber Middleware Stack (+8 more)

### Community 19 - "ServerInterface"
Cohesion: 0.05
Nodes (26): AcceptPactParams, ApproveCheckInParams, ConfirmPayoutParams, CreatePactParams, DeclareRestParams, DisputeCheckInParams, ListPactCheckInsParams, MarkPayoutPaidParams (+18 more)

### Community 20 - "github.com/google/uuid.UUID"
Cohesion: 0.10
Nodes (14): Service, Queries, Payout, ListDecisionsRow, GetCheckInForMemberParams, ListCheckInsForPactParams, ListMembersForPactsRow, ListPactMembersRow (+6 more)

### Community 21 - "AGENTS.md operating manual"
Cohesion: 0.22
Nodes (11): Buku Tabungan (passbook visual world), AGENTS.md operating manual, Append-only ledger invariant (balance = SUM(ledger_entries.amount)), Definition of done (every PR), One PLAN task per branch/PR workflow, Design workflow and direction (docs/design/README.md), Design quality loop (screenshots, impeccable detect, finish review, documenter), Wise DESIGN.md reference (money UI craft only) (+3 more)

### Community 22 - "deploy/compose.yaml base compose"
Cohesion: 0.21
Nodes (11): deploy/compose.yaml base compose, garage v2.4.1 service, mailpit service, Phase 0: Repository foundation (done), Task 4.1 BlobStore drivers (s3, fs), Task 7.2 compose.dev/prod and Caddy, Root package.json Bun scripts (infra:up, codegen, dev:*, test, lint, deploy:*), garage:init bootstrap script (scripts/garage-init.ts) (+3 more)

### Community 23 - "compilerOptions"
Cohesion: 0.15
Nodes (12): compilerOptions, allowImportingTsExtensions, module, moduleResolution, noEmit, noUncheckedIndexedAccess, skipLibCheck, strict (+4 more)

### Community 24 - "service package (use-cases, tx boundaries)"
Cohesion: 0.19
Nodes (12): ADR-0004 asynq on Redis, River (Postgres queue) fallback, ADR-0008 Three Run Modes via Bun Scripts, Compose profiles (infra/app/edge/tools/backup), scripts/dev.ts orchestrator, asynq Priority Queues (critical/default/media), BlobStore Interface (s3 and fs drivers), domain package (pure, no I/O) (+4 more)

### Community 25 - "PLAN.md implementation plan"
Cohesion: 0.29
Nodes (11): Idempotency key per coin movement in same tx, Single transition implementation in internal/service, pure rules in internal/domain, redis:8 service (AOF), PLAN.md implementation plan, Task 1.3 Domain: terms, deadlines, check-in FSM, ledger math, Task 1.5 Services: pact lifecycle, Task 1.6 Check-in transitions and settlement, Task 1.7 Auth (argon2id, Redis sessions, CSRF, rate limit) (+3 more)

### Community 26 - "Upload and Compression Pipeline"
Cohesion: 0.19
Nodes (12): ADR-0005 Garage Storage, Presigned Uploads, Server Compression, attachments table, check_ins table, Media Queue Backpressure (MEDIA_QUEUE_MAX), Presigned PUT Upload, proofs table, Tiptap 3 Rich Text Editor, UPLOAD_MODE=proxy fallback (+4 more)

### Community 28 - "ADR-0002 Append-only Ledger with Idempotency Keys"
Cohesion: 0.33
Nodes (9): ADR-0002 Append-only Ledger with Idempotency Keys, forbid_mutation append-only trigger, ledger_entries table, Redis Aggregate Cache (pot balance, Today), tepatictl Admin CLI, Invariant L1: balance = SUM(ledger), Invariant L2: ledger never updated/deleted, Invariant L3: unique idempotency_key (+1 more)

### Community 29 - "Check-in State Machine"
Cohesion: 0.36
Nodes (8): ADR-0007 Review Power Model, Stamp violet for human decisions, decisions table, Check-in State Machine, Dispute, Late Edits and Resubmission, Override, Rest Day

### Community 30 - "Direction contract (THESIS/OWN-WORLD/STORY/FIRST VIEWPORT/SIGNATURE MOVE/FORM/FINISH)"
Cohesion: 0.29
Nodes (7): Surface brief: authenticated app shell, First viewport (countdown header, Hari ini, Perlu ditinjau, mini passbook), Own-world palette (cover teal, stamp violet, highlighter yellow, member colours), Passbook print signature move, Task 5.2 Design tokens and primitives (Amount, Countdown, StatusChip, MemberLine), Task 6.2 Today screen, Task 6.4 Pact page: passbook and calendar

### Community 31 - "Contract-first (openapi.yaml, goose migration, sqlc, codegen)"
Cohesion: 0.33
Nodes (6): Contract-first (openapi.yaml, goose migration, sqlc, codegen), sqlc.yaml config, internal/store generated package (pgx/v5, uuid/time overrides), postgres:18 service, Task 1.4 Store layer (sqlc), Task 2.1 OpenAPI contract (next unchecked)

### Community 32 - "Diagrams README (archify)"
Cohesion: 0.33
Nodes (6): Upload hardening (presigned, sniffed type, EXIF strip, server limits), Diagrams README (archify), tepati-architecture.html, checkin-lifecycle.html, upload-sequence.html, Task 4.2 Upload intent, complete, media processing

### Community 33 - "server_test.go"
Cohesion: 0.12
Nodes (39): ApproveCheckIn200JSONResponse, RejectCheckIn200JSONResponse, newEnv(), TestAccessLogHasContextAndNoBodies(), TestAuthRoutesHaveAStricterLimit(), TestBodyOverOneMegabyteIsRejected(), TestClientIPIsNeverEmpty(), TestCORSIsDevOnly() (+31 more)

### Community 34 - "StrictServerInterface"
Cohesion: 0.05
Nodes (17): AcceptPactResponseObject, ConfirmPayoutResponseObject, CreatePactRequestObject, CreatePactResponseObject, DeclareRestRequestObject, DeclareRestResponseObject, DisputeCheckInResponseObject, GetPactRequestObject (+9 more)

### Community 35 - "Injected Clock Interface"
Cohesion: 0.50
Nodes (4): Injected Clock Interface, Test Clock Endpoint (/_test/clock), Testing Strategy, Non-functional Requirements

### Community 36 - "testing.T"
Cohesion: 0.11
Nodes (30): newAuth(), TestLoginRateLimit(), TestRegisterLoginLogout(), TestRequireUserMiddleware(), TestNormaliseEmail(), CodeOf(), TestCodeOf(), TestEffectStringFallback() (+22 more)

### Community 38 - "MemberView"
Cohesion: 0.09
Nodes (21): Role, clampLimit(), MemberView, PactView, Service, toMemberView(), ListReviewQueuePageRow, ListTodayCheckInsRow (+13 more)

### Community 47 - "User"
Cohesion: 0.07
Nodes (16): GetMe200JSONResponse, Login200JSONResponse, LoginRequest, ProposeRequest, RegisterRequest, RegisterRequestLocale, UserLocale, GetMeRequestObject (+8 more)

### Community 48 - "wise.DESIGN.md"
Cohesion: 0.06
Nodes (32): Border Radius Scale, Brand & Accent, Brand Accent — Tertiary, Breakpoints, Buttons, Cards & Containers, Colors, Components (+24 more)

### Community 49 - "Problem"
Cohesion: 0.07
Nodes (15): ApproveCheckIn4XXApplicationProblemPlusJSONResponse, ApproveCheckIndefaultApplicationProblemPlusJSONResponse, CreatePactdefaultApplicationProblemPlusJSONResponse, DisputeCheckIn4XXApplicationProblemPlusJSONResponse, FieldError, GetPact4XXApplicationProblemPlusJSONResponse, GetReviewQueue4XXApplicationProblemPlusJSONResponse, GetToday4XXApplicationProblemPlusJSONResponse (+7 more)

### Community 50 - "Pact"
Cohesion: 0.17
Nodes (15): checkDraftShape(), doerKey(), encodeTerms(), DraftInput, Service, hashToken(), TestAgreementFlowSchedulesPactFundsPotAndGeneratesCheckIns(), inviteUsable() (+7 more)

### Community 51 - "CreateUploadRequestObject"
Cohesion: 0.10
Nodes (10): CompleteUploadParams, CreateUpload201JSONResponse, CreateUploadParams, CompleteUploadRequestObject, CompleteUploadResponseObject, CreateUploadRequestObject, CreateUploadResponseObject, GetAttachmentRequestObject (+2 more)

### Community 52 - "GetReviewQueueParams"
Cohesion: 0.09
Nodes (12): GetReviewQueueParams, ListLedgerParams, ListNotificationsParams, ListPactsParams, GetReviewQueueRequestObject, GetReviewQueueResponseObject, ListLedgerRequestObject, ListLedgerResponseObject (+4 more)

### Community 53 - "api.gen.go"
Cohesion: 0.08
Nodes (15): AcceptRequest, CompleteUpload200JSONResponse, GetAttachment200JSONResponse, GetToday200JSONResponse, Logout204Response, MarkNotificationsRead204Response, MarkPaidRequest, MarkReadRequest (+7 more)

### Community 54 - "spec_test.go"
Cohesion: 0.14
Nodes (14): TestEveryContractOperationIsRouted(), authenticate(), isPublic(), contains(), loadSpec(), TestEveryGoErrorCodeIsInTheContract(), TestProblemCodesDeclaredPerOperationExist(), TestPublicRoutesMatchSpecSecurity() (+6 more)

### Community 55 - "at"
Cohesion: 0.21
Nodes (20): at(), fixture, proof(), TestConcurrentSweepsChargeEachMissExactlyOnce(), TestDisputeUpholdDismissAndReversal(), TestMembershipAndPactState(), TestOverrideLimitAndPower(), TestPotNeverDropsBelowFloor() (+12 more)

### Community 56 - "Handlers"
Cohesion: 0.12
Nodes (8): AcceptPact200JSONResponse, CreatePact201JSONResponse, GetPact200JSONResponse, JoinInvite200JSONResponse, ProposePact200JSONResponse, UpdatePact200JSONResponse, draftInput(), Handlers

### Community 57 - "fixture"
Cohesion: 0.10
Nodes (6): FakeClock, NewFakeClock(), TestFakeClock(), fixture, env, syncBuffer

### Community 59 - "stub"
Cohesion: 0.15
Nodes (8): MarkNotificationsReadParams, ApproveCheckInRequestObject, ApproveCheckInResponseObject, MarkNotificationsReadRequestObject, MarkNotificationsReadResponseObject, RejectCheckInRequestObject, RejectCheckInResponseObject, stub

### Community 60 - "dev.ts"
Cohesion: 0.19
Nodes (15): ADR-0008, appSpecs(), loadEnv(), Mode, nativePreflight(), server, startApps(), web (+7 more)

### Community 61 - ".GetReviewQueue"
Cohesion: 0.16
Nodes (8): GetReviewQueue200JSONResponse, ListLedger200JSONResponse, ListNotifications200JSONResponse, ListPacts200JSONResponse, decodeCursor(), encodeCursor(), limitOf(), Handlers

### Community 62 - "CheckInDetail"
Cohesion: 0.15
Nodes (11): Decision, DecisionAction, ProofVersion, CheckInAction, CheckInDetail, Proof, apiCheckInDetail(), apiProof() (+3 more)

### Community 63 - "Pact"
Cohesion: 0.19
Nodes (10): InvitePreview, PactPage, PactStatus, Role, TodayPact, Pact, PactDraft, PactMember (+2 more)

### Community 64 - "env.ts"
Cohesion: 0.25
Nodes (12): cleanValue(), EnvMap, generateSecret(), materialize(), MaterializeOptions, parseEnv(), SecretKind, setEnvValue() (+4 more)

### Community 65 - "log/slog.Logger"
Cohesion: 0.16
Nodes (9): main(), main(), Main(), discardLogger(), errorHandler(), writeProblem(), Serve(), New() (+1 more)

### Community 66 - "run"
Cohesion: 0.20
Nodes (11): run(), Storage, DatabaseCheck(), RedisCheck(), StorageCheck(), DefaultLimits(), Connect(), NewStore() (+3 more)

### Community 67 - "db.ts"
Cohesion: 0.18
Nodes (9): @aws-sdk/client-s3, [action, ...rest], dir, tools, paths, ROOT, expected, names (+1 more)

### Community 68 - "me"
Cohesion: 0.41
Nodes (3): invalid(), Handlers, me()

### Community 69 - "New"
Cohesion: 0.18
Nodes (7): withClientIP(), registerOps(), newMetrics(), corsDev(), jsonBodies(), New(), metrics

### Community 70 - "setup.test.ts"
Cohesion: 0.18
Nodes (6): COLORS, pipeLines(), ProcSpec, supervise(), SuperviseOptions, tmp

### Community 71 - ".JoinInvite"
Cohesion: 0.20
Nodes (4): JoinInvitedefaultApplicationProblemPlusJSONResponse, JoinInviteParams, JoinInviteRequestObject, JoinInviteResponseObject

### Community 74 - "observe"
Cohesion: 0.21
Nodes (5): clientIP(), observe(), opsPath(), rateLimiters(), routeSet

### Community 75 - "Tepati: project status and handoff"
Cohesion: 0.17
Nodes (11): 1. Where we are, 2. Branch and merge state, 3. How to verify the current state, 4. What exists, 5. Decisions and rulings made so far, 6. Known gaps and deferred work, 7. Brief for Phase 3, Backend (`apps/server`, Go module `github.com/andi-frame/lockedin/apps/server`) (+3 more)

### Community 76 - "github.com/oapi-codegen/runtime/types.UUID"
Cohesion: 0.24
Nodes (8): AttachmentKind, AttachmentStatus, AttachmentUrls, ProofRequest, UploadIntent, UploadIntentRequest, Attachment, apiAttachment()

### Community 77 - "CheckIn"
Cohesion: 0.24
Nodes (8): CheckInList, CheckInStatus, ReviewQueueItem, ReviewQueuePage, TodayCheckIn, CheckIn, MemberRef, apiCheckIn()

### Community 79 - ".MarkPayoutPaid"
Cohesion: 0.25
Nodes (5): ConfirmPayout200JSONResponse, MarkPayoutPaid200JSONResponse, Payout, Handlers, apiPayout()

### Community 80 - "LedgerLine"
Cohesion: 0.33
Nodes (7): LedgerKind, LedgerPage, LedgerLine, apiDate(), apiDatePtr(), apiLedgerLine(), apiLedgerLines()

### Community 81 - ".PreviewInvite"
Cohesion: 0.31
Nodes (4): PreviewInvite200JSONResponse, PreviewInviteRequestObject, PreviewInviteResponseObject, apiRef()

### Community 82 - "ADR Index and Supersede Process"
Cohesion: 0.22
Nodes (8): ADR-0006 PostgreSQL with sqlc and goose, ADR-0009 Visual Direction: Buku Tabungan, Passbook Print signature move, ADR-0010: API conventions: errors, idempotency, bodies, pagination, Consequences, Context, Decision, ADR Index and Supersede Process

### Community 83 - "codegen.ts"
Cohesion: 0.25
Nodes (8): check, goApi, server, spawn(), spec, tool(), tools, tsSchema

### Community 84 - ".GetToday"
Cohesion: 0.29
Nodes (4): GetTodayRequestObject, GetTodayResponseObject, ptr(), ptrInt()

### Community 86 - "idempotency"
Cohesion: 0.32
Nodes (5): uploadsUnavailable(), digest(), idempotency(), replay(), problemError()

### Community 87 - "apiTerms"
Cohesion: 0.29
Nodes (6): Evidence, MemberTerms, MemberTermsRole, Terms, apiTerms(), termsFromAPI()

### Community 88 - "schema.d.ts"
Cohesion: 0.29
Nodes (6): components, $defs, operations, paths, webhooks, RFC-9457

### Community 89 - "RegisterHandlersWithOptions"
Cohesion: 0.40
Nodes (5): FiberServerOptions, HandlerMiddlewareFunc, MiddlewareFunc, RegisterHandlers(), RegisterHandlersWithOptions()

### Community 90 - "User"
Cohesion: 0.40
Nodes (3): normaliseEmail(), User, RegisterInput

### Community 91 - "Notification"
Cohesion: 0.50
Nodes (4): NotificationKind, NotificationPage, Notification, apiNotification()

### Community 94 - "ErrorCode"
Cohesion: 0.67
Nodes (3): ErrorCode, problem(), toProblem()

## Knowledge Gaps
- **156 isolated node(s):** `github.com/andi-frame/lockedin/apps/server`, `ctxUser`, `AcceptRequest`, `MarkPaidRequest`, `MarkReadRequest` (+151 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 302 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **75 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `ServerInterfaceWrapper` connect `ServerInterfaceWrapper` to `.GetMe`, `.GetToday`, `.ListPacts`, `.Login`, `.Logout`, `.MarkPayoutPaid`, `.Register`, `.JoinInvite`, `.ResolveDispute`, `strictHandler`, `.ListLedger`, `ServerInterface`, `api.gen.go`, `RegisterHandlersWithOptions`, `fiber.Ctx`, `.AcceptPact`, `.DisputeCheckIn`?**
  _High betweenness centrality (0.032) - this node is a cross-community bridge._
- **What connects `github.com/andi-frame/lockedin/apps/server`, `ctxUser`, `AcceptRequest` to the rest of the system?**
  _156 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `sampleTerms` be split into smaller, more focused modules?**
  _Cohesion score 0.09782608695652174 - nodes in this community are weakly interconnected._
- **Why does `fixture` connect `fixture` to `api_integration_test.go`, `newStack`, `testing.T`, `context.Context`, `User`?**
  _High betweenness centrality (0.020) - this node is a cross-community bridge._
- **Should `api_integration_test.go` be split into smaller, more focused modules?**
  _Cohesion score 0.09300385778333854 - nodes in this community are weakly interconnected._
- **Why does `Date` connect `Date` to `api_integration_test.go`, `time.Time`?**
  _High betweenness centrality (0.013) - this node is a cross-community bridge._
- **Should `CheckIn` be split into smaller, more focused modules?**
  _Cohesion score 0.11711711711711711 - nodes in this community are weakly interconnected._