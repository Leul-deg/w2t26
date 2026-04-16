# Test Coverage & README Audit Report

**Project:** Library Operations & Enrollment Management Suite (LMS)
**Audited:** 2026-04-16
**Note:** Report saved to `/home/leul/Documents/w2t26/.tmp/` (root `/.tmp/` not writable in this environment)

---

# PART 1 — TEST COVERAGE AUDIT

## 1. Backend Endpoint Inventory

All routes registered under `/api/v1` prefix via `registerRoutes()` in `backend/cmd/server/main.go:267`.

### Complete Endpoint List (103 total)

| # | Method | Path | Domain |
|---|--------|------|--------|
| 1 | GET | `/api/v1/health` | health |
| 2 | GET | `/api/v1/ready` | health |
| 3 | POST | `/api/v1/auth/login` | users/auth |
| 4 | GET | `/api/v1/auth/me` | users/auth |
| 5 | POST | `/api/v1/auth/logout` | users/auth |
| 6 | POST | `/api/v1/auth/stepup` | users/auth |
| 7 | GET | `/api/v1/users` | users/management |
| 8 | POST | `/api/v1/users` | users/management |
| 9 | GET | `/api/v1/users/:id` | users/management |
| 10 | PATCH | `/api/v1/users/:id` | users/management |
| 11 | POST | `/api/v1/users/:id/roles` | users/management |
| 12 | DELETE | `/api/v1/users/:id/roles/:role_id` | users/management |
| 13 | POST | `/api/v1/users/:id/branches` | users/management |
| 14 | DELETE | `/api/v1/users/:id/branches/:branch_id` | users/management |
| 15 | GET | `/api/v1/readers/statuses` | readers |
| 16 | GET | `/api/v1/readers` | readers |
| 17 | POST | `/api/v1/readers` | readers |
| 18 | GET | `/api/v1/readers/:id` | readers |
| 19 | PATCH | `/api/v1/readers/:id` | readers |
| 20 | PATCH | `/api/v1/readers/:id/status` | readers |
| 21 | GET | `/api/v1/readers/:id/history` | readers |
| 22 | GET | `/api/v1/readers/:id/holdings` | readers |
| 23 | POST | `/api/v1/readers/:id/reveal` | readers |
| 24 | GET | `/api/v1/holdings` | holdings |
| 25 | POST | `/api/v1/holdings` | holdings |
| 26 | GET | `/api/v1/holdings/:id` | holdings |
| 27 | PATCH | `/api/v1/holdings/:id` | holdings |
| 28 | DELETE | `/api/v1/holdings/:id` | holdings |
| 29 | GET | `/api/v1/holdings/:id/copies` | holdings |
| 30 | POST | `/api/v1/holdings/:id/copies` | holdings |
| 31 | GET | `/api/v1/copies/statuses` | copies |
| 32 | GET | `/api/v1/copies/lookup` | copies |
| 33 | GET | `/api/v1/copies/:id` | copies |
| 34 | PATCH | `/api/v1/copies/:id` | copies |
| 35 | PATCH | `/api/v1/copies/:id/status` | copies |
| 36 | POST | `/api/v1/circulation/checkout` | circulation |
| 37 | POST | `/api/v1/circulation/return` | circulation |
| 38 | GET | `/api/v1/circulation` | circulation |
| 39 | GET | `/api/v1/circulation/copy/:id` | circulation |
| 40 | GET | `/api/v1/circulation/reader/:id` | circulation |
| 41 | GET | `/api/v1/circulation/active/:id` | circulation |
| 42 | GET | `/api/v1/stocktake` | stocktake |
| 43 | POST | `/api/v1/stocktake` | stocktake |
| 44 | GET | `/api/v1/stocktake/:id` | stocktake |
| 45 | PATCH | `/api/v1/stocktake/:id/status` | stocktake |
| 46 | GET | `/api/v1/stocktake/:id/findings` | stocktake |
| 47 | POST | `/api/v1/stocktake/:id/scan` | stocktake |
| 48 | GET | `/api/v1/stocktake/:id/variances` | stocktake |
| 49 | GET | `/api/v1/programs` | programs |
| 50 | POST | `/api/v1/programs` | programs |
| 51 | GET | `/api/v1/programs/:id` | programs |
| 52 | PATCH | `/api/v1/programs/:id` | programs |
| 53 | PATCH | `/api/v1/programs/:id/status` | programs |
| 54 | GET | `/api/v1/programs/:id/prerequisites` | programs |
| 55 | POST | `/api/v1/programs/:id/prerequisites` | programs |
| 56 | DELETE | `/api/v1/programs/:id/prerequisites/:req_id` | programs |
| 57 | GET | `/api/v1/programs/:id/rules` | programs |
| 58 | POST | `/api/v1/programs/:id/rules` | programs |
| 59 | DELETE | `/api/v1/programs/:id/rules/:rule_id` | programs |
| 60 | POST | `/api/v1/programs/:id/enroll` | enrollment |
| 61 | GET | `/api/v1/programs/:id/enrollments` | enrollment |
| 62 | GET | `/api/v1/programs/:id/seats` | enrollment |
| 63 | GET | `/api/v1/enrollments/:id` | enrollment |
| 64 | POST | `/api/v1/enrollments/:id/drop` | enrollment |
| 65 | GET | `/api/v1/enrollments/:id/history` | enrollment |
| 66 | GET | `/api/v1/readers/:reader_id/enrollments` | enrollment |
| 67 | GET | `/api/v1/imports` | imports |
| 68 | POST | `/api/v1/imports` | imports |
| 69 | GET | `/api/v1/imports/template/:type` | imports |
| 70 | GET | `/api/v1/imports/:id` | imports |
| 71 | POST | `/api/v1/imports/:id/commit` | imports |
| 72 | POST | `/api/v1/imports/:id/rollback` | imports |
| 73 | GET | `/api/v1/imports/:id/errors.csv` | imports |
| 74 | GET | `/api/v1/exports` | exports |
| 75 | POST | `/api/v1/exports/readers` | exports |
| 76 | POST | `/api/v1/exports/holdings` | exports |
| 77 | GET | `/api/v1/content` | content |
| 78 | POST | `/api/v1/content` | content |
| 79 | GET | `/api/v1/content/:id` | content |
| 80 | PATCH | `/api/v1/content/:id` | content |
| 81 | POST | `/api/v1/content/:id/submit` | content |
| 82 | POST | `/api/v1/content/:id/retract` | content |
| 83 | POST | `/api/v1/content/:id/publish` | content |
| 84 | POST | `/api/v1/content/:id/archive` | content |
| 85 | GET | `/api/v1/moderation/queue` | moderation |
| 86 | GET | `/api/v1/moderation/items/:id` | moderation |
| 87 | POST | `/api/v1/moderation/items/:id/assign` | moderation |
| 88 | POST | `/api/v1/moderation/items/:id/decide` | moderation |
| 89 | GET | `/api/v1/feedback` | feedback |
| 90 | POST | `/api/v1/feedback` | feedback |
| 91 | GET | `/api/v1/feedback/tags` | feedback |
| 92 | GET | `/api/v1/feedback/:id` | feedback |
| 93 | POST | `/api/v1/feedback/:id/moderate` | feedback |
| 94 | GET | `/api/v1/appeals` | appeals |
| 95 | POST | `/api/v1/appeals` | appeals |
| 96 | GET | `/api/v1/appeals/:id` | appeals |
| 97 | POST | `/api/v1/appeals/:id/review` | appeals |
| 98 | POST | `/api/v1/appeals/:id/arbitrate` | appeals |
| 99 | GET | `/api/v1/reports/definitions` | reports |
| 100 | GET | `/api/v1/reports/run` | reports |
| 101 | GET | `/api/v1/reports/aggregates` | reports |
| 102 | POST | `/api/v1/reports/recalculate` | reports |
| 103 | GET | `/api/v1/reports/export` | reports |

---

## 2. API Test Mapping Table

**Test type legend:**
- **True No-Mock HTTP** = real Echo HTTP stack + real PostgreSQL, no service/repo mocks
- **HTTP with Mocking** = HTTP layer invoked but services/repos mocked
- **Non-HTTP Unit** = handler/service called directly, no HTTP routing

| # | Endpoint | Covered | Test Type | Test File(s) | Evidence |
|---|----------|---------|-----------|--------------|----------|
| 1 | GET /api/v1/health | YES | True No-Mock HTTP | `health_test.go` | `TestHealth_Liveness` |
| 2 | GET /api/v1/ready | YES | True No-Mock HTTP | `health_test.go` | `TestHealth_Readiness` |
| 3 | POST /api/v1/auth/login | YES | True No-Mock HTTP | `auth_test.go` | `TestAuth_LoginSuccess`, `TestAuth_LoginFailedBadPassword`, `TestAuth_FailedAttemptCounter`, `TestAuth_LockoutAfterFiveFailures`, `TestAuth_CaptchaRequired` |
| 4 | GET /api/v1/auth/me | **NO** | — | — | No test calls this endpoint |
| 5 | POST /api/v1/auth/logout | PARTIAL | True No-Mock HTTP | `auth_test.go` | `TestAuth_SessionRequired_Returns401` (error path only; success path untested) |
| 6 | POST /api/v1/auth/stepup | YES | True No-Mock HTTP | `auth_test.go` | `TestAuth_StepUpRevealFails_WrongPassword`, `TestAuth_StepUpRevealAllowsReaderLookup` |
| 7 | GET /api/v1/users | YES | True No-Mock HTTP | `authz_test.go` | `TestUsers_UnassignedBranch_UserListIsEmpty` |
| 8 | POST /api/v1/users | **NO** | — | — | `createTestUser()` inserts directly via `userRepo.Create()`, bypasses HTTP |
| 9 | GET /api/v1/users/:id | YES | True No-Mock HTTP | `authz_test.go` | `TestUsers_GetUser_CrossBranch_Returns404` |
| 10 | PATCH /api/v1/users/:id | YES (error path) | True No-Mock HTTP | `authz_test.go` | `TestUsers_UpdateUser_CrossBranch_Returns404` (404 path; success path untested) |
| 11 | POST /api/v1/users/:id/roles | **NO** | — | — | No test found |
| 12 | DELETE /api/v1/users/:id/roles/:role_id | **NO** | — | — | No test found |
| 13 | POST /api/v1/users/:id/branches | **NO** | — | — | `assignUserToBranch()` uses direct SQL, no HTTP |
| 14 | DELETE /api/v1/users/:id/branches/:branch_id | **NO** | — | — | No test found |
| 15 | GET /api/v1/readers/statuses | YES | True No-Mock HTTP | `readers_test.go` | `TestReaders_ListStatuses` |
| 16 | GET /api/v1/readers | YES | True No-Mock HTTP | `readers_test.go`, `auth_test.go` | `TestReaders_List_AdminSeesAll`, `TestReader_NoPermission_Returns403`, `TestReaders_ListWithSearch` |
| 17 | POST /api/v1/readers | YES | True No-Mock HTTP | `readers_test.go` | `TestReaders_CreateAndGet`, `TestReaders_Create_DuplicateReaderNumber_Returns409` |
| 18 | GET /api/v1/readers/:id | YES | True No-Mock HTTP | `readers_test.go`, `auth_test.go` | `TestReaders_CreateAndGet`, `TestAuth_BranchScope`, `TestAuth_MaskedFields`, `TestReader_CrossBranch_Returns404` |
| 19 | PATCH /api/v1/readers/:id | YES | True No-Mock HTTP | `readers_test.go` | `TestReaders_Update` |
| 20 | PATCH /api/v1/readers/:id/status | YES | True No-Mock HTTP | `readers_test.go` | `TestReaders_UpdateStatus` |
| 21 | GET /api/v1/readers/:id/history | YES | True No-Mock HTTP | `readers_test.go` | `TestReaders_GetLoanHistory` |
| 22 | GET /api/v1/readers/:id/holdings | YES | True No-Mock HTTP | `readers_test.go` | `TestReaders_GetCurrentHoldings` |
| 23 | POST /api/v1/readers/:id/reveal | YES | True No-Mock HTTP | `auth_test.go` | `TestAuth_StepUpRevealFails_WrongPassword`, `TestAuth_StepUpRevealAllowsReaderLookup` |
| 24 | GET /api/v1/holdings | YES | True No-Mock HTTP | `holdings_copies_test.go`, `domain_perms_test.go` | `TestHoldings_List`, `TestHoldings_AdminCanList`, `TestHoldings_ListRequiresPermission` |
| 25 | POST /api/v1/holdings | YES | True No-Mock HTTP | `holdings_copies_test.go`, `domain_perms_test.go` | `TestHoldings_CreateAndGet`, `TestHoldings_CreateRequiresPermission` |
| 26 | GET /api/v1/holdings/:id | YES | True No-Mock HTTP | `holdings_copies_test.go` | `TestHoldings_CreateAndGet`, `TestHoldings_GetUnknown_Returns404` |
| 27 | PATCH /api/v1/holdings/:id | YES | True No-Mock HTTP | `holdings_copies_test.go` | `TestHoldings_Update` |
| 28 | DELETE /api/v1/holdings/:id | YES | True No-Mock HTTP | `holdings_copies_test.go` | `TestHoldings_Deactivate` |
| 29 | GET /api/v1/holdings/:id/copies | YES | True No-Mock HTTP | `holdings_copies_test.go` | `TestCopies_ListForHolding` |
| 30 | POST /api/v1/holdings/:id/copies | YES | True No-Mock HTTP | `holdings_copies_test.go`, `domain_perms_test.go` | `TestCopies_AddAndGet`, `TestHoldings_AddCopy_CrossBranch_Returns404` |
| 31 | GET /api/v1/copies/statuses | YES | True No-Mock HTTP | `holdings_copies_test.go` | `TestCopies_Statuses` |
| 32 | GET /api/v1/copies/lookup | YES | True No-Mock HTTP | `holdings_copies_test.go` | `TestCopies_LookupByBarcode` |
| 33 | GET /api/v1/copies/:id | YES | True No-Mock HTTP | `holdings_copies_test.go` | `TestCopies_AddAndGet` |
| 34 | PATCH /api/v1/copies/:id | YES | True No-Mock HTTP | `holdings_copies_test.go` | `TestCopies_UpdateCondition` |
| 35 | PATCH /api/v1/copies/:id/status | YES | True No-Mock HTTP | `holdings_copies_test.go` | `TestCopies_UpdateStatus` |
| 36 | POST /api/v1/circulation/checkout | YES | True No-Mock HTTP | `circulation_test.go`, `authz_test.go` | `TestCirculation_CheckoutAndReturn`, `TestCirculation_DoubleCheckout_Returns409`, `TestCirculation_Checkout_CrossBranch_Returns404`, `TestCirculation_CheckoutNoPermission_Returns403` |
| 37 | POST /api/v1/circulation/return | YES | True No-Mock HTTP | `circulation_test.go`, `authz_test.go` | `TestCirculation_CheckoutAndReturn`, `TestCirculation_Return_NoCopy_Returns404`, `TestCirculation_Return_CrossBranch_Returns404` |
| 38 | GET /api/v1/circulation | YES | True No-Mock HTTP | `circulation_test.go`, `authz_test.go` | `TestCirculation_List`, `TestCirculation_AdminListAllBranches` |
| 39 | GET /api/v1/circulation/copy/:id | YES | True No-Mock HTTP | `circulation_test.go` | `TestCirculation_ListByCopy` |
| 40 | GET /api/v1/circulation/reader/:id | YES | True No-Mock HTTP | `circulation_test.go` | `TestCirculation_ListByReader` |
| 41 | GET /api/v1/circulation/active/:id | YES | True No-Mock HTTP | `circulation_test.go`, `authz_test.go` | `TestCirculation_CheckoutAndReturn`, `TestCirculation_ActiveCheckout_NoCopy_Returns404`, `TestCirculation_ActiveCheckout_CrossBranch_Returns404` |
| 42 | GET /api/v1/stocktake | YES | True No-Mock HTTP | `stocktake_lifecycle_test.go`, `domain_perms_test.go` | `TestStocktake_ListSessions`, `TestStocktake_AdminCanList`, `TestStocktake_ListRequiresPermission` |
| 43 | POST /api/v1/stocktake | YES | True No-Mock HTTP | `stocktake_lifecycle_test.go`, `domain_perms_test.go` | `TestStocktake_CreateAndGetSession`, `TestStocktake_DuplicateActiveSession_Returns409`, `TestStocktake_NoPermission_Returns403` |
| 44 | GET /api/v1/stocktake/:id | YES | True No-Mock HTTP | `stocktake_lifecycle_test.go` | `TestStocktake_CreateAndGetSession` |
| 45 | PATCH /api/v1/stocktake/:id/status | YES | True No-Mock HTTP | `stocktake_lifecycle_test.go` | `TestStocktake_CloseSession`, `TestStocktake_CancelSession` |
| 46 | GET /api/v1/stocktake/:id/findings | YES | True No-Mock HTTP | `stocktake_lifecycle_test.go` | `TestStocktake_ListFindings` |
| 47 | POST /api/v1/stocktake/:id/scan | YES | True No-Mock HTTP | `stocktake_lifecycle_test.go` | `TestStocktake_RecordScan_KnownBarcode`, `TestStocktake_RecordScan_UnknownBarcode`, `TestStocktake_RecordScan_Idempotent` |
| 48 | GET /api/v1/stocktake/:id/variances | YES | True No-Mock HTTP | `stocktake_lifecycle_test.go` | `TestStocktake_GetVariances` |
| 49 | GET /api/v1/programs | YES | True No-Mock HTTP | `programs_enrollment_test.go`, `authz_test.go` | `TestPrograms_List`, `TestPrograms_AdminListAllBranches` |
| 50 | POST /api/v1/programs | YES | True No-Mock HTTP | `programs_enrollment_test.go` | `TestPrograms_CreateAndGet` |
| 51 | GET /api/v1/programs/:id | YES | True No-Mock HTTP | `programs_enrollment_test.go` | `TestPrograms_CreateAndGet`, `TestPrograms_GetUnknown_Returns404` |
| 52 | PATCH /api/v1/programs/:id | YES | True No-Mock HTTP | `programs_enrollment_test.go` | `TestPrograms_Update` |
| 53 | PATCH /api/v1/programs/:id/status | YES | True No-Mock HTTP | `programs_enrollment_test.go` | `TestPrograms_UpdateStatus` |
| 54 | GET /api/v1/programs/:id/prerequisites | YES | True No-Mock HTTP | `programs_enrollment_test.go` | `TestPrograms_Prerequisites` |
| 55 | POST /api/v1/programs/:id/prerequisites | YES | True No-Mock HTTP | `programs_enrollment_test.go` | `TestPrograms_Prerequisites` |
| 56 | DELETE /api/v1/programs/:id/prerequisites/:req_id | YES | True No-Mock HTTP | `programs_enrollment_test.go` | `TestPrograms_Prerequisites` |
| 57 | GET /api/v1/programs/:id/rules | YES | True No-Mock HTTP | `programs_enrollment_test.go`, `authz_test.go` | `TestPrograms_AddAndRemoveRule`, `TestPrograms_Rules_CrossBranch_Returns404` |
| 58 | POST /api/v1/programs/:id/rules | YES | True No-Mock HTTP | `programs_enrollment_test.go`, `authz_test.go` | `TestPrograms_AddAndRemoveRule`, `TestPrograms_AddRule_CrossBranch_Returns404` |
| 59 | DELETE /api/v1/programs/:id/rules/:rule_id | YES | True No-Mock HTTP | `programs_enrollment_test.go`, `authz_test.go` | `TestPrograms_AddAndRemoveRule`, `TestPrograms_RemoveRule_CrossBranch_Returns404` |
| 60 | POST /api/v1/programs/:id/enroll | YES | True No-Mock HTTP | `programs_enrollment_test.go` | `TestEnrollment_EnrollAndDrop`, `TestEnrollment_DoubleEnroll_Returns409`, `TestEnrollment_OverCapacity_Returns422` |
| 61 | GET /api/v1/programs/:id/enrollments | YES | True No-Mock HTTP | `programs_enrollment_test.go` | `TestEnrollment_EnrollAndDrop` |
| 62 | GET /api/v1/programs/:id/seats | YES | True No-Mock HTTP | `programs_enrollment_test.go` | `TestEnrollment_EnrollAndDrop` |
| 63 | GET /api/v1/enrollments/:id | YES | True No-Mock HTTP | `programs_enrollment_test.go` | `TestEnrollment_EnrollAndDrop` |
| 64 | POST /api/v1/enrollments/:id/drop | YES | True No-Mock HTTP | `programs_enrollment_test.go` | `TestEnrollment_EnrollAndDrop` |
| 65 | GET /api/v1/enrollments/:id/history | YES | True No-Mock HTTP | `programs_enrollment_test.go` | `TestEnrollment_History` |
| 66 | GET /api/v1/readers/:reader_id/enrollments | YES | True No-Mock HTTP | `programs_enrollment_test.go` | `TestEnrollment_EnrollAndDrop` |
| 67 | GET /api/v1/imports | YES | True No-Mock HTTP | `imports_lifecycle_test.go`, `domain_perms_test.go` | `TestImports_ListJobs_Returns200`, `TestImports_AdminCanList`, `TestImports_ListRequiresPermission` |
| 68 | POST /api/v1/imports | YES | True No-Mock HTTP | `imports_lifecycle_test.go`, `domain_perms_test.go` | `TestImports_UploadValidCSV_ReturnsPreviewReady`, `TestImports_UploadInvalidCSV_Returns422`, `TestImports_UploadNoPermission_Returns403` |
| 69 | GET /api/v1/imports/template/:type | **NO** | — | — | No test found for this endpoint |
| 70 | GET /api/v1/imports/:id | YES | True No-Mock HTTP | `imports_lifecycle_test.go` | `TestImports_GetPreview_Returns200` |
| 71 | POST /api/v1/imports/:id/commit | YES | True No-Mock HTTP | `imports_lifecycle_test.go` | `TestImports_CommitValidJob_InsertsRows`, `TestImports_CommitFailedJob_Returns422` |
| 72 | POST /api/v1/imports/:id/rollback | YES | True No-Mock HTTP | `imports_lifecycle_test.go` | `TestImports_Rollback_Returns200` |
| 73 | GET /api/v1/imports/:id/errors.csv | YES | True No-Mock HTTP | `imports_lifecycle_test.go` | `TestImports_DownloadErrors_Returns200` |
| 74 | GET /api/v1/exports | YES | True No-Mock HTTP | `domain_perms_test.go` | `TestExports_AdminCanList`, `TestExports_ListRequiresPermission`, `TestExports_ContentModeratorCannotList` |
| 75 | POST /api/v1/exports/readers | **NO** (HTTP) | Non-HTTP Unit | `exports/handler_test.go` | `TestExportReaders_MissingPermission` calls `h.ExportReaders(c)` directly — no HTTP routing |
| 76 | POST /api/v1/exports/holdings | **NO** | — | — | No test found; exports/handler_test.go only covers ExportReaders permission |
| 77 | GET /api/v1/content | YES | True No-Mock HTTP | `content_moderation_test.go` | `TestContent_List` |
| 78 | POST /api/v1/content | YES | True No-Mock HTTP | `content_moderation_test.go` | `TestContent_CreateAndGet`, `TestContent_FullLifecycle_SubmitApprovePublish`, `TestContent_NoPermission_Returns403` |
| 79 | GET /api/v1/content/:id | YES | True No-Mock HTTP | `content_moderation_test.go` | `TestContent_CreateAndGet`, `TestContent_GetUnknown_Returns404` |
| 80 | PATCH /api/v1/content/:id | YES | True No-Mock HTTP | `content_moderation_test.go` | `TestContent_Update` |
| 81 | POST /api/v1/content/:id/submit | YES | True No-Mock HTTP | `content_moderation_test.go` | `TestContent_FullLifecycle_SubmitApprovePublish`, `TestContent_Retract_Returns200` |
| 82 | POST /api/v1/content/:id/retract | YES | True No-Mock HTTP | `content_moderation_test.go` | `TestContent_Retract_Returns200` |
| 83 | POST /api/v1/content/:id/publish | YES | True No-Mock HTTP | `content_moderation_test.go` | `TestContent_FullLifecycle_SubmitApprovePublish`, `TestContent_Reject_BlocksPublish` |
| 84 | POST /api/v1/content/:id/archive | YES | True No-Mock HTTP | `content_moderation_test.go` | `TestContent_FullLifecycle_SubmitApprovePublish` |
| 85 | GET /api/v1/moderation/queue | YES | True No-Mock HTTP | `content_moderation_test.go` | `TestContent_FullLifecycle_SubmitApprovePublish`, `TestContent_Reject_BlocksPublish` |
| 86 | GET /api/v1/moderation/items/:id | YES | True No-Mock HTTP | `content_moderation_test.go` | `TestContent_FullLifecycle_SubmitApprovePublish` |
| 87 | POST /api/v1/moderation/items/:id/assign | YES | True No-Mock HTTP | `content_moderation_test.go` | `TestContent_FullLifecycle_SubmitApprovePublish` |
| 88 | POST /api/v1/moderation/items/:id/decide | YES | True No-Mock HTTP | `content_moderation_test.go` | `TestContent_FullLifecycle_SubmitApprovePublish`, `TestContent_Reject_BlocksPublish` |
| 89 | GET /api/v1/feedback | YES | True No-Mock HTTP | `feedback_appeals_test.go`, `domain_perms_test.go` | `TestFeedback_List` |
| 90 | POST /api/v1/feedback | YES | True No-Mock HTTP | `feedback_appeals_test.go`, `domain_perms_test.go` | `TestFeedback_SubmitAndGet`, `TestFeedback_NoSubmitPermission_Returns403`, `TestFeedback_OperationsStaffCanSubmit` |
| 91 | GET /api/v1/feedback/tags | YES | True No-Mock HTTP | `feedback_appeals_test.go` | `TestFeedback_ListTags` |
| 92 | GET /api/v1/feedback/:id | YES | True No-Mock HTTP | `feedback_appeals_test.go` | `TestFeedback_SubmitAndGet` |
| 93 | POST /api/v1/feedback/:id/moderate | YES | True No-Mock HTTP | `feedback_appeals_test.go` | `TestFeedback_ModerateApprove`, `TestFeedback_ModerateReject` |
| 94 | GET /api/v1/appeals | YES | True No-Mock HTTP | `feedback_appeals_test.go`, `domain_perms_test.go` | `TestAppeals_List` |
| 95 | POST /api/v1/appeals | YES | True No-Mock HTTP | `feedback_appeals_test.go`, `domain_perms_test.go` | `TestAppeals_SubmitAndGet`, `TestAppeals_SubmitRequiresPermission`, `TestAppeals_OperationsStaffCanSubmit` |
| 96 | GET /api/v1/appeals/:id | YES | True No-Mock HTTP | `feedback_appeals_test.go` | `TestAppeals_SubmitAndGet` |
| 97 | POST /api/v1/appeals/:id/review | YES | True No-Mock HTTP | `feedback_appeals_test.go` | `TestAppeals_Review_TransitionsStatus`, `TestAppeals_Arbitrate_Upheld` |
| 98 | POST /api/v1/appeals/:id/arbitrate | YES | True No-Mock HTTP | `feedback_appeals_test.go` | `TestAppeals_Arbitrate_Upheld`, `TestAppeals_Arbitrate_Dismissed`, `TestAppeals_NoArbitratePermission_Returns403` |
| 99 | GET /api/v1/reports/definitions | YES | True No-Mock HTTP | `domain_perms_test.go` | `TestReports_AdminCanListDefinitions`, `TestReports_ListDefinitionsRequiresRead`, `TestReports_OperationsStaffCanListDefinitions` |
| 100 | GET /api/v1/reports/run | YES | True No-Mock HTTP | `domain_perms_test.go`, `authz_test.go` | `TestReports_RunReport_ReturnsDefinitionAndRows`, `TestReports_RunReportRequiresRead`, `TestReports_AdminRunWithBranchParam_Returns200` |
| 101 | GET /api/v1/reports/aggregates | **NO** | — | — | No test found |
| 102 | POST /api/v1/reports/recalculate | YES | True No-Mock HTTP | `authz_test.go` | `TestReports_AdminRecalculateAllBranches_Returns200` |
| 103 | GET /api/v1/reports/export | YES | True No-Mock HTTP | `domain_perms_test.go` | `TestReports_Export_ReturnsCSVAndAuditHeader` |

---

## 3. API Test Classification

### Category 1: True No-Mock HTTP Tests
**File location:** `backend/API_TESTS/`

All 14 test files in this directory use `httptest.NewRequest()` routed through `app.e.ServeHTTP()`. Services and repositories are real Go structs backed by real PostgreSQL (via `testdb.Open(t)` → `DATABASE_TEST_URL`). No service mocking, no `vi.mock`, no `sinon.stub`, no dependency injection overrides. Business logic executes end-to-end.

Files: `auth_test.go`, `authz_test.go`, `schema_test.go`, `domain_perms_test.go`, `complete_app_test.go`, `health_test.go`, `helpers_test.go`, `readers_test.go`, `holdings_copies_test.go`, `circulation_test.go`, `programs_enrollment_test.go`, `imports_lifecycle_test.go`, `stocktake_lifecycle_test.go`, `content_moderation_test.go`, `feedback_appeals_test.go`

### Category 2: HTTP with Mocking
None detected. All HTTP-reaching backend tests use real dependencies.

### Category 3: Non-HTTP (Unit / Integration without HTTP)

**Handler permission tests** (`backend/internal/domain/*/handler_test.go`):
- `holdings/handler_test.go` — calls `h.ListHoldings(c)`, `h.CreateHolding(c)` etc. directly; service is `nil`; no HTTP routing
- `stocktake/handler_test.go` — same nil-service pattern
- `circulation/handler_test.go` — same nil-service pattern
- `users/handler_admin_test.go` — `h.ListUsers(c)` called directly; no HTTP routing
- `exports/handler_test.go` — `h.ExportReaders(c)` and `h.ExportHoldings(c)` called directly

**Service unit tests** (`backend/internal/domain/*/service_test.go`):
- `appeals/service_test.go` — stub repo, no HTTP
- `content/service_test.go` — stub repo, no HTTP
- `moderation/service_test.go` — stub repo, no HTTP
- `feedback/service_test.go` — stub repo, no HTTP
- `reports/service_test.go` — stub repo, no HTTP
- `enrollment/service_test.go` — stub repo, no HTTP
- `imports/service_test.go` — stub repo, no HTTP
- `config/config_test.go` — no DB, no HTTP

**Schema/constraint tests** (`backend/API_TESTS/schema_test.go`):
- Direct SQL execution via `pgxpool`; no HTTP layer

**Frontend tests** (all 13 files in `frontend/src/`):
- Use `vi.mock()` for API clients and auth context
- Test component rendering and user interactions only
- No real HTTP calls; all API responses mocked

---

## 4. Coverage Summary

| Metric | Count | Percentage |
|--------|-------|------------|
| Total endpoints | 103 | — |
| Endpoints with HTTP tests | 92 | **89.3%** |
| Endpoints with TRUE no-mock HTTP | 92 | **89.3%** |
| Endpoints with no HTTP coverage | 11 | 10.7% |

**Note:** Endpoints #5 (POST /logout) and #10 (PATCH /users/:id) are counted as HTTP-covered because they are called via HTTP in tests, even though only the failure path is exercised.

---

## 5. Uncovered Endpoints (11)

| # | Endpoint | Reason |
|---|----------|--------|
| 4 | GET /api/v1/auth/me | Never called in any test |
| 8 | POST /api/v1/users | `createTestUser()` uses `userRepo.Create()` directly, no API call |
| 11 | POST /api/v1/users/:id/roles | `assignUserRole()` uses direct SQL, no API call |
| 12 | DELETE /api/v1/users/:id/roles/:role_id | No test found |
| 13 | POST /api/v1/users/:id/branches | `assignUserToBranch()` uses direct SQL, no API call |
| 14 | DELETE /api/v1/users/:id/branches/:branch_id | No test found |
| 69 | GET /api/v1/imports/template/:type | No test found |
| 75 | POST /api/v1/exports/readers | Only tested via direct handler call (`h.ExportReaders(c)`) — HTTP routing bypassed |
| 76 | POST /api/v1/exports/holdings | No test found |
| 101 | GET /api/v1/reports/aggregates | No test found |
| — | POST /api/v1/auth/logout (success path) | Success path never tested; only 401 error path covered |

---

## 6. Mock Detection Results

**Backend API tests:** No mocking detected. Evidence:
- `complete_app_test.go:46–85`: All repos are real `postgres.New*Repo(pool)` instances
- `auth_test.go:44–79`: `newTestApp()` wires real `postgres.UserRepo`, `postgres.SessionRepo`
- No `gomock`, `testify/mock`, or stub injection patterns found in `API_TESTS/`

**Handler unit tests:** Nil-service pattern used (direct handler call without HTTP).
- `exports/handler_test.go:49`: `h := exports.NewHandler(nil)` then `h.ExportReaders(c)` directly
- `holdings/handler_test.go`: `h := holdings.NewHandler(nil)` pattern
- These are classified as Non-HTTP Unit tests, not HTTP tests

**Service unit tests:** Stub repos (struct with hardcoded return values). Not API tests.

**Frontend tests:** `vi.mock()` used for all API modules and auth context.
- `CirculationPage.test.tsx:8–16`: `vi.mock('../../api/circulation', ...)` — all API calls mocked

---

## 7. Unit Test Summary

### Service Tests Covered
| Service | Test File | Key Coverage |
|---------|-----------|--------------|
| appeals | `appeals/service_test.go` | State machine: submitted→under_review→resolved/dismissed |
| content | `content/service_test.go` | State machine: draft→pending_review→approved/rejected→published→archived |
| moderation | `moderation/service_test.go` | Queue assignment and decision logic |
| feedback | `feedback/service_test.go` | Tag resolution, status transitions |
| reports | `reports/service_test.go` | Filtering, aggregate recalculation |
| enrollment | `enrollment/service_test.go` | Concurrency safety, capacity enforcement |
| imports | `imports/service_test.go` | Validation rules, rollback |
| config | `config/config_test.go` | Config loading, env var resolution |

### Handler Permission Tests Covered
| Handler | Test File |
|---------|-----------|
| holdings | `holdings/handler_test.go` |
| stocktake | `stocktake/handler_test.go` |
| circulation | `circulation/handler_test.go` |
| users (admin) | `users/handler_admin_test.go` |
| exports | `exports/handler_test.go` |

### Important Modules NOT Unit-Tested

| Module | Location | Gap |
|--------|----------|-----|
| readers service | `internal/domain/readers/service.go` | No `service_test.go`; only tested end-to-end via API |
| holdings service | `internal/domain/holdings/service.go` | No `service_test.go` |
| circulation service | `internal/domain/circulation/service.go` | No `service_test.go` |
| programs service | `internal/domain/programs/service.go` | No `service_test.go` |
| stocktake service | `internal/domain/stocktake/service.go` | No `service_test.go` |
| exports service | `internal/domain/exports/service.go` | No `service_test.go` |
| users service | `internal/domain/users/service.go` | No `service_test.go` |
| crypto package | `internal/crypto/` | No test file |
| middleware (RBAC, branch scope) | `internal/middleware/` | No test files |
| audit logger | `internal/audit/` | No test file; exercised indirectly |
| scheduler | `internal/scheduler/` | No test file |

---

## 8. API Observability Check

**Verdict: STRONG**

Most API tests verify:
- Explicit HTTP method and path in `doRequest()` calls
- Expected status codes via `require.Equal` / `assert.Equal`
- Response body content via JSON unmarshalling and field assertions

**Examples of strong observability:**
- `readers_test.go:115–120`: Asserts `id`, `first_name`, `last_name`, `reader_number` in response body
- `circulation_test.go:54–56`: Asserts `copy_id` and `reader_id` in checkout response
- `domain_perms_test.go:522–526`: Asserts `definition.name`, `rows`, `row_count` in report response
- `imports_lifecycle_test.go:95–99`: Asserts `status="preview_ready"` and `error_count=0`
- `imports_lifecycle_test.go:274–281`: Verifies actual DB rows inserted after commit

**Weak points:**
- `programs_enrollment_test.go:191`: `DELETE /programs/:id/rules/:rule_id` only asserts 200, not deletion confirmation
- `content_moderation_test.go:82–93`: `GET /content` asserts only `items` key exists, not content

---

## 9. Test Quality & Sufficiency

### Strengths
- All HTTP tests exercise real PostgreSQL with real business logic — no mock drift
- Auth/RBAC tested with multiple roles and explicit 401/403 scenarios per domain
- Branch-scope isolation tested as cross-branch 404 (not 403) enforcement
- Conflict scenarios tested: double checkout (409), duplicate enrollment (409), over-capacity (422), duplicate barcode (409), duplicate active stocktake session (409)
- Full lifecycle tests: content moderation pipeline (draft→submit→review→approve→publish→archive) tested in a single test
- Import pipeline tested end-to-end including DB row verification post-commit
- DB constraint tests in `schema_test.go` verify uniqueness, FK, and check constraints directly

### Weaknesses
- User management CRUD (create/role-assign/branch-assign) not HTTP-tested; test helpers bypass the API
- Export triggers (POST /exports/readers, /exports/holdings) not tested at HTTP layer
- GET /auth/me never called; no test verifies session data returned on the me endpoint
- No success path for logout
- GET /reports/aggregates not tested
- No E2E frontend→backend tests; frontend tests mock all API calls
- AES-256 encryption round-trip with real data not tested (documented in README Known Limitations)

### run_tests.sh Assessment
**CORRECT:** The script uses Docker Compose (`docker compose exec -T backend/frontend`). All test commands execute inside containers. No local dependency required to run tests.
- Backend tests: `go test ./internal/...` and `go test ./API_TESTS/... -v` executed in container
- Frontend tests: `npm run test` executed in container
- Clean database reset performed before each run

---

## 10. End-to-End Expectations

This is a fullstack project. True E2E (frontend browser → real backend) tests are absent. The README documents this explicitly: "Frontend browser behavior (TypeScript compiles; not E2E tested)".

**Partial compensation:** Strong API integration tests compensate for missing E2E at the HTTP boundary. Frontend tests verify component logic but not backend integration.

---

## 11. Test Coverage Score

### Score: **74 / 100**

### Score Rationale

| Factor | Weight | Earned | Notes |
|--------|--------|--------|-------|
| HTTP endpoint coverage (92/103) | 25 | 22 | 89.3% coverage; deducted for 11 uncovered endpoints |
| True no-mock API quality | 25 | 24 | All HTTP tests are true no-mock against real DB; minor deduction for missing success-path tests on logout/me |
| Test depth (auth, permissions, edge cases) | 20 | 18 | Excellent auth/RBAC testing; conflict/idempotency scenarios covered; minor deduction for shallow assertions on some endpoints |
| Unit test completeness | 15 | 7 | 8 service tests + 5 handler tests; but readers/holdings/circulation/programs/stocktake/exports/users services lack unit tests; crypto/middleware/scheduler untested |
| E2E / frontend integration | 10 | 0 | No E2E tests; frontend tests mock all API calls |
| run_tests.sh quality | 5 | 5 | Docker-based, correct, complete |

### Key Gaps
1. **User management API completely untested at HTTP level** — POST /users, role/branch assignment endpoints. All test helpers bypass the API via direct DB/repo calls.
2. **Export triggers (POST /exports/readers, /exports/holdings)** — handler_test.go only tests permission logic via direct function call, not HTTP routing.
3. **GET /auth/me never called** — session data endpoint not verified.
4. **GET /reports/aggregates** — no test at any level.
5. **GET /imports/template/:type** — template download untested.
6. **No frontend-to-backend integration** — all 13 frontend tests mock API calls.
7. **~7 service modules lack unit tests** — readers, holdings, circulation, programs, stocktake, exports, users services tested only indirectly via API.

### Confidence & Assumptions
- **High confidence** on endpoint inventory: extracted directly from handler source files and main.go
- **High confidence** on test mapping: all test files read in full
- **Assumption:** `schema_test.go` constraint tests are treated as separate from API tests (they test DB directly, not via HTTP)
- **Assumption:** Partial coverage of POST /logout (error path) is counted as HTTP-tested for the endpoint row, but noted as incomplete
- **Static analysis only:** No tests were executed; coverage is inferred from code inspection

---

---

# PART 2 — README AUDIT

## Project Type Detection

**Declared type:** Fullstack (Go backend + React TypeScript frontend)
**Evidence:** README line 1: "Library Operations & Enrollment Management Suite (LMS)" with sections for both backend (`go run ./cmd/server`) and frontend (`npm run dev`)
**Inference required:** No — project type is clearly stated.

---

## README Location

**Path:** `/home/leul/Documents/w2t26/repo/README.md`
**Exists:** YES (507 lines)

---

## Hard Gate Evaluation

### Gate 1: Formatting
**Status: PASS**
README uses clean markdown: tables, code blocks, headers, horizontal rules. Readable and well-structured.

---

### Gate 2: Startup Instructions — `docker-compose up`
**Status: FAIL**

The Quick Start section (lines 9–30) requires:
```bash
psql -U postgres -c "CREATE USER lms_user WITH PASSWORD 'changeme';"
psql -U postgres -c "CREATE DATABASE lms OWNER lms_user;"
psql -U postgres -c "CREATE DATABASE lms_test OWNER lms_user;"
cd backend && cp .env.example .env
mkdir -p /etc/lms && openssl rand -out /etc/lms/lms.key 32 && chmod 600 /etc/lms/lms.key
go run ./cmd/migrate up && go run ./cmd/server
cd frontend && npm install && npm run dev
```

**`docker-compose.yml` exists** at the project root and fully orchestrates postgres, backend, and frontend. However, **the README does not mention Docker Compose at all** across all 507 lines. There is no `docker-compose up` instruction anywhere in the README.

**Impact:** A reviewer following the README must install Go, Node.js, and PostgreSQL locally. The Docker-based startup path is completely undocumented.

---

### Gate 3: Access Method
**Status: PASS**
- Frontend: `http://localhost:3000` (line 29, 173)
- Backend API: `http://localhost:8080` (line 144)
- Ports and URLs clearly stated.

---

### Gate 4: Verification Method
**Status: PASS**
curl commands provided:
```bash
curl http://localhost:8080/api/v1/health
curl -s -c /tmp/lms.txt -X POST http://localhost:8080/api/v1/auth/login ...
curl -s -b /tmp/lms.txt http://localhost:8080/api/v1/auth/me | jq .
```
UI verification: "Sign in with admin / Admin1234!" (line 175)

---

### Gate 5: Environment Rules (Strict Docker Containment)
**Status: FAIL**

The README explicitly instructs the reviewer to run:
- `go mod download` (line 93) — local Go dependency install
- `npm install` (line 101) — local Node dependency install
- Manual PostgreSQL setup via `psql` commands
- Manual file system operations (`mkdir -p /etc/lms`, `openssl rand ...`)

None of these are Docker-contained. The README presents the project as requiring local tool installation.

**Note:** `run_tests.sh` IS Docker-based but the README does not prominently feature or link to it for the startup path.

---

### Gate 6: Demo Credentials (Conditional)
**Status: PARTIAL FAIL**

Auth exists. README provides (lines 181–184):

| Username | Password | Role |
|----------|----------|------|
| `admin` | `Admin1234!` | Administrator |

**Issues:**
1. **Only one credential pair provided** — admin only. The system has three distinct roles: `administrator`, `operations_staff`, `content_moderator`.
2. Creating accounts for other roles requires manual SQL insertion (lines 187–199 show the raw INSERT statements required).
3. No ready-to-use credentials for `operations_staff` or `content_moderator` — a reviewer cannot test role-based behavior without writing SQL.

Under the strict requirement: "Must provide username/email + password for ALL roles" — **FAIL for non-admin roles**.

---

## High Priority Issues

| # | Issue | Location | Impact |
|---|-------|----------|--------|
| 1 | No `docker-compose up` instruction | Quick Start, entire README | Reviewer must install Go, Node.js, PostgreSQL locally to run the system |
| 2 | `npm install` required as a manual step | Line 101 | Violates strict Docker containment rule |
| 3 | Manual PostgreSQL setup required | Lines 51–57 | Requires local Postgres installation; no containerized alternative shown |
| 4 | Only admin credentials seeded | Lines 181–184 | Reviewer cannot test operations_staff or content_moderator roles without writing SQL |

---

## Medium Priority Issues

| # | Issue | Location | Impact |
|---|-------|----------|--------|
| 5 | `run_tests.sh` (Docker-based one-command test runner) not mentioned in README | Entire README | Reviewer may not discover the automated test runner |
| 6 | No API reference beyond curl examples | Lines 159–165 | No endpoint inventory, request/response schemas, or error codes documented |
| 7 | `go mod download` required manually | Line 93 | Local Go toolchain required |
| 8 | Creating non-admin users requires raw SQL | Lines 187–199 | Poor reviewer experience for RBAC validation |

---

## Low Priority Issues

| # | Issue | Location | Impact |
|---|-------|----------|--------|
| 9 | `CRYPTO_KEY_FILE` setup is manual and complex | Lines 63–69 | Requires understanding of file paths and openssl; Docker Compose handles this automatically but isn't documented |
| 10 | Implementation Status section distinguishes verified/unverified but doesn't clearly guide reviewer to what IS testable | Lines 304–348 | May cause reviewer confusion about scope |
| 11 | `run_tests.sh` is listed in git root but README doesn't describe it | — | Discovery requires inspecting repository directly |

---

## Hard Gate Failures Summary

| Gate | Status | Description |
|------|--------|-------------|
| Formatting | PASS | Clean markdown |
| Startup — `docker-compose up` | **FAIL** | Not mentioned in README; local toolchain required |
| Access Method | PASS | URL + port documented |
| Verification Method | PASS | curl commands provided |
| Environment Rules | **FAIL** | `npm install`, `go mod download`, `psql` commands required |
| Demo Credentials | **PARTIAL FAIL** | Admin credentials only; other roles require raw SQL |

---

## README Verdict: **PARTIAL PASS**

**Justification:**
The README is comprehensive, well-written, and demonstrates strong engineering documentation quality (507 lines covering security controls, implementation status, known limitations, and verification boundaries). However, it fails two hard gates and partially fails a third:

1. **FAIL:** No `docker-compose up` startup path documented. Despite a working `docker-compose.yml` that fully orchestrates the system, all startup instructions require local tool installation.
2. **FAIL:** Environment setup requires local tools (`npm install`, `go mod download`, manual PostgreSQL).
3. **PARTIAL FAIL:** Only admin credentials seeded for login; other roles require manual SQL to set up.

These failures mean a reviewer following the README cannot start the system with a single Docker command. The project would need a Docker-based startup section to pass strict mode.

---

# COMBINED FINAL SUMMARY

| Section | Score / Verdict |
|---------|----------------|
| **Test Coverage Score** | **74 / 100** |
| **README Verdict** | **PARTIAL PASS** |

**Test coverage** is notably strong in API quality (all HTTP tests are true no-mock against real PostgreSQL), but weakened by incomplete user management coverage, missing export trigger tests, absent unit tests for several service modules, and no E2E frontend integration.

**README** contains excellent technical depth but fails the containerization requirement and provides incomplete demo credentials for multi-role testing.
