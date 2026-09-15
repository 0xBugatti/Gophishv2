# Ultimate GoPhish — Master Implementation Plan

> **Source data:** All 723 open issues + all open PRs from `gophish/gophish` as of 2026-03-26, cross-referenced against the full feature inventory of this fork.
> **GoPhish upstream status:** Effectively unmaintained since Dec 2021 (last merge) / Sep 2022 (last release v0.12.1). 723 open issues, 0 recent merges. All PRs below are community contributions that upstream never merged.
> **This file is planning-only. Do not edit code while referencing it.**

---

## Upstream Project Context

| Stat | Value |
|------|-------|
| Stars | 13,668 |
| Forks | 2,850 |
| Open Issues | 723 |
| Last Release | v0.12.1 — September 2022 |
| Last Merged PR | December 2021 |
| Go Version in Upstream | 1.13 (we use 1.24 ✅) |
| GORM Version | v1 / jinzhu (deprecated) |
| SQLite Driver | CGO-based mattn/go-sqlite3 |

---

## What Is Already Done in This Fork (Skip These)

| Feature | Origin |
|---------|--------|
| SMS Campaigns (Twilio/Nexmo) | Fork-original |
| MFA Simulation — SMS / Email OTP / TOTP | Fork-original |
| Campaign Sets (multi-campaign launch) | Fork-original |
| QR Code Generator | Fork-original + PR #2897 |
| Async Reports Engine (DOCX / XLSX) | Fork-original |
| IMAP Monitor + Non-Campaign Reports | Fork-original + PR #1894 |
| Scenarios (reusable playbooks) | Fork-original |
| Teams + RBAC (role-based access) | Fork-original + issue #2612 |
| AES-256-GCM Credential Encryption | Fork-original |
| Dark Themes (teal / crimson) | Fork-original |
| Timezone-aware campaign scheduling | Fork-original |
| Evilginx integration routing | Fork-original |
| Activity Detection DB schema | Fork-original |
| Multi-stage Docker + docker-compose | Fork-original + PR #9381 |
| HTML redirect page after credential submit | Fork-original + PR #2651 |
| Go 1.24 (upstream still on 1.13) | Fork-original + PR #5810 |
| IMAP `/imap/{id}` route (bug fix) | This session |
| `api.teamId` + `api.user` JS (bug fix) | This session |
| Docker ca-certificates in runtime stage | This session (closes issue #2889) |
| Dynamic redirect URL (`{{.RId}}` etc.) | Already in `models/page.go:98` ValidateTemplate (closes #3310) |
| Session cookie persistent DoS (partial) | Partial via encryption key |

---

## Phase 1 — Critical Security Fixes
> Fix these before implementing any new feature. All are open issues in upstream.

### 1.1 Session Not Invalidated on Logout
- **Source:** Issue #9368
- **Severity:** Critical — persistent account takeover
- **Detail:** The `_gophish` session cookie remains valid indefinitely after logout. If an attacker captures the cookie (or another tab is left open), they retain full admin access even after password change. Server never destroys the session server-side.
- **Fix:**
  - Store active session tokens in a `sessions` DB table (token hash → user_id, expires_at)
  - On logout: DELETE the session row
  - On every authenticated request: verify session row exists and is not expired
  - Add configurable `session_timeout_hours` to config.json
- **Files:** `controllers/admin.go`, `middleware/middleware.go`, `models/user.go`, new migration

### 1.2 API Key Exposed in DOM + Not Invalidated on Reset
- **Source:** Issues #9366, #2213, PR #2864
- **Severity:** Critical — API key is rendered in plaintext HTML/JS on every page load; accessible to any XSS payload or browser extension
- **⚠️ Cross-cutting risk:** Every JS file in this fork reads `user.api_key` from the DOM-injected object (`base.html:34`). Fixing this means changing the auth mechanism in all 21 custom JS modules (`gophish.js`, `sms_templates.js`, `teams.js`, `scenarios.js`, `non_campaign_reports.js`, `landing_pages.js`, `campaign_sets.js`, etc.). Must be done in a dedicated branch touching only auth plumbing.
- **Fix:**
  - Remove `api_key` from `base.html` user object; fetch it once via `GET /api/users/me` after page load and store in a JS closure (never in DOM)
  - Store API key as bcrypt hash in DB, not plaintext
  - On key reset: immediately update DB hash; all in-flight requests with old key get 401
  - Update all `user.api_key` references in every JS file to use the closure-stored value
- **Files:** `templates/base.html`, `controllers/admin.go`, `models/user.go`, `middleware/middleware.go`, all `static/js/src/app/*.js`

### 1.3 User Enumeration via Login Endpoint
- **Source:** Issue #9349
- **Severity:** High — valid usernames take 100–120ms, non-existent 50–60ms, enabling targeted brute-force
- **Fix:**
  - Always perform bcrypt comparison regardless of whether user exists (use dummy hash for non-existent users)
  - Generic error message: "Invalid username or password" — never distinguish which field is wrong
  - Add configurable rate limiting + account lockout on `/login` (e.g., 5 attempts → 15 min lockout)
- **Files:** `controllers/admin.go`, `models/user.go`

### 1.4 Pre-Auth SSRF in URI Handler
- **Source:** Issue #3269
- **Severity:** High — unauthenticated server-side request forgery
- **Detail:** Phishing server follows redirects from URI parameters without authentication, enabling internal network scanning.
- **Fix:**
  - Validate all redirect target URLs before following
  - Block RFC-1918 private IP ranges (10.x, 172.16–31.x, 192.168.x, 127.x)
  - Block metadata service IPs (169.254.169.254 for AWS/GCP/Azure)
- **Files:** `controllers/phish.go`

### 1.5 Race Condition — Duplicate Landing Page Names
- **Source:** Issue #3278
- **Severity:** Medium — concurrent requests bypass uniqueness check creating duplicate pages
- **Fix:**
  - Add `UNIQUE INDEX` on `(user_id, name)` in pages table migration
  - Handle DB constraint violation with 409 Conflict response
- **Files:** `db/db_sqlite3/migrations/`, `db/db_mysql/migrations/`, `models/page.go`

### 1.6 Stored XSS on Campaign Delete Button
- **Source:** PR #2991
- **Severity:** Medium — campaign name rendered unescaped in delete confirmation
- **Fix:** Apply `escapeHtml()` to campaign name in Swal delete confirmation in `campaigns.js`
- **Files:** `static/js/src/app/campaigns.js`

### 1.7 GoPhish Fails to Start if Admin Account Renamed/Deleted
- **Source:** Issues #2487, #1963
- **Severity:** Medium — can permanently brick the instance
- **Detail:** GoPhish startup checks for an account named exactly `admin`. Renaming it causes startup failure with cryptic error.
- **Fix:**
  - Change startup check to look for any user with `PermissionModifySystem`, not by username
  - If none found, create a new admin account with random password (print to logs)
- **Files:** `models/models.go` (or wherever startup admin check lives)

~~### 1.8 Docker Image Missing CA Certificates~~ ✅ **ALREADY DONE**
> `Dockerfile:39` already installs `ca-certificates` in the runtime stage. No action needed.

### 1.9 Landing Page Form Action Attribute Stripped on Import
- **Source:** Issue #3338
- **Severity:** Critical (silent) — credential capture silently fails for any landing page imported from a real site that uses `action=""` or a relative URL. Campaign runs to completion, zero credentials captured, no error shown.
- **Detail:** The HTML sanitiser/parser in the "Import Site" and manual paste flow strips `action` attributes from `<form>` tags when the value is empty or relative. GoPhish rewrites absolute action URLs to its own `/post` handler, but relative/empty ones are dropped, so the form POSTs nowhere.
- **Fix:**
  - In `controllers/phish.go` (or wherever landing page HTML is parsed): preserve `action=""` and relative `action` paths; rewrite them to GoPhish's credential capture endpoint just like absolute URLs
  - Add a validation warning in the landing page save API if no `<form action=...>` pointing to GoPhish is detected
- **Files:** `controllers/phish.go`, `controllers/api/page.go`, `models/page.go`

### 1.10 `account_locked` Column Missing on Migrated Databases
- **Source:** Issue #2481
- **Severity:** High — crashes GoPhish at runtime on any DB that pre-dates the `account_locked` migration
- **Detail:** Code references `users.account_locked` but the migration that adds this column was introduced after many deployments. Upgrading GoPhish without re-running migrations causes `Error 1054: Unknown column 'account_locked'`.
- **Fix:**
  - Add a `IF NOT EXISTS` guard or proper migration version check
  - Ensure `account_locked` migration runs idempotently on upgrade
- **Files:** `db/db_sqlite3/migrations/`, `db/db_mysql/migrations/`, `models/user.go`

### 1.11 SMTP From Field Rejects `Name <email>` Format
- **Source:** Issue #2443
- **Severity:** Medium — operators cannot use the standard `Display Name <address>` format; it is silently garbled or rejected
- **Fix:**
  - Parse From field using `mail.ParseAddress()` before storing; validate and re-serialise correctly
  - Ensure gomail receives the display name and address as separate arguments, not a raw combined string
- **Files:** `models/smtp.go`, `static/js/src/app/sending_profiles.js`

---

## Phase 2 — Core Engine Reliability
> Bugs and technical debt in the sending/tracking pipeline.

### 2.1 Event Persistence — Status Advances Even When Event Write Fails
- **Source:** PR #9392, Issue #9391
- **Detail:** `createEvent()` returns an error that callers ignore; result `status` and `modified_date` are still updated even when no event row was written. Dashboard (results) and timeline export can diverge.
- **Fix:**
  - Wrap event insert + result status update in a single DB transaction
  - If event insert fails → rollback both → return error to caller
  - Add `ORDER BY time ASC, id ASC` to timeline event queries for determinism
- **Files:** `models/result.go`, `controllers/phish.go`

### 2.2 Send Rate + Interval Control per SMTP Profile _(merged 2.2 + 2.3)_
- **Source:** PR #9394, Issue #8701, PR #5809, PR #1557
- **Note:** These were two separate items achieving the same goal — controlling send speed. A single config covers both: rate limiter (token bucket) sets the ceiling; interval is just a fixed-delay expression of the same knob.
- **Fix:**
  - Add `send_rate` field (emails/minute, 0 = unlimited) to SMTP model; apply `golang.org/x/time/rate` token-bucket limiter in worker per SMTP profile
  - Add `send_interval_ms int` to Campaign model as optional override (fixed delay between dispatches, takes precedence over rate limiter if set)
  - Sending Profiles UI: "Max send rate (emails/min)" field
  - Campaign modal: "Delay between emails (ms)" optional field
  - SQLite + MySQL migrations for both fields
- **Files:** `models/smtp.go`, `models/campaign.go`, `worker/worker.go`, migrations, `static/js/src/app/sending_profiles.js`, `static/js/src/app/campaigns.js`

### 2.4 Parallel / Concurrent Email Sending
- **Source:** Issue #3316, Issue #501
- **Fix:**
  - Add `send_concurrency int` to Campaign (default: 1, max: configurable)
  - Replace serial send loop with `errgroup`-based worker pool
  - Each goroutine gets its own SMTP connection from pool
- **Files:** `worker/worker.go`, `models/campaign.go`

### 2.5 Random Mail Sending Order
- **Source:** PR #9345, Issues #3167, #351
- **Fix:**
  - Add `randomize_send_order bool` to Campaign
  - Before dispatch, shuffle targets slice with `crypto/rand`-seeded Fisher-Yates
- **Files:** `models/campaign.go`, `worker/worker.go`

### 2.6 GORM v2 + Goose v3 Migration
- **Source:** PR #9382
- **Detail:** `github.com/jinzhu/gorm` (v1) is unmaintained. `bitbucket.org/liamstask/goose` is abandoned.
- **⚠️ Isolated branch required:** This fork has 13 custom models (sms.go, mfa.go, team.go, scenario.go, campaign_set.go, qr_code.go, report.go, imap.go extended, etc.) all written in GORM v1 syntax. GORM v2 changed `.Find`, `.Where`, `.Save`, associations, and hooks API. A careless upgrade silently breaks query logic. **Must run in a dedicated branch with full test coverage; do not merge until every model is verified.**
- **Fix:**
  - Migrate all model files to `gorm.io/gorm` v2 API
  - Replace `goose` with `github.com/pressly/goose/v3`
  - Update all `.Find()`, `.Save()`, `.Where()`, hook (`BeforeSave`/`AfterFind`) patterns to v2 syntax
- **Files:** All `models/*.go`, `models/models.go`, `go.mod`

### 2.7 Pure-Go SQLite (Remove CGO Dependency)
- **Source:** PR #3063
- **Fix:** Replace `github.com/mattn/go-sqlite3` with `modernc.org/sqlite` (pure Go, no CGO, enables simple cross-compilation)
- **Files:** `go.mod`, `models/models.go`

### 2.8 HTTP/HTTPS Proxy Support
- **Source:** PR #3339, Issue #3077
- **Fix:**
  - Read standard `HTTP_PROXY` / `HTTPS_PROXY` env vars
  - Pass custom transport to gomail dialer
  - Document in README
- **Files:** `models/smtp.go`, `docker/run.sh`

### 2.9 Kubernetes CSRF Token Failure Across Pod Restarts
- **Source:** Issue #2107
- **Detail:** K8s kills/recreates pods; session state is in-memory and lost; users get CSRF token invalid errors.
- **Fix:**
  - Make CSRF secret key configurable via env var (already partially done with encryption key)
  - Store sessions in DB (ties into fix 1.1)
  - Document K8s deployment pattern with persistent CSRF key
- **Files:** `config/config.go`, `middleware/middleware.go`

### 2.10 DB Connection Pool Configuration
- **Source:** Issue #2529
- **Detail:** DB connection pool hardcoded to 1; high-volume deployments (2,400+ daily) hit contention.
- **Fix:**
  - Add `db_max_open_conns`, `db_max_idle_conns` to config.json
  - Pass to `db.DB().SetMaxOpenConns()` / `SetMaxIdleConns()` in models.go
- **Files:** `config/config.go`, `models/models.go`

---

## Phase 3 — Email Campaign Features
> New sending capabilities from open PRs and long-standing feature requests.

### 3.1 CC Email Support
- **Source:** PR #9376, Issue #2852
- **Fix:** Add `cc` field (comma-separated) to Template model; set CC headers in gomail; UI field in email templates modal
- **Files:** `models/template.go`, `models/maillog.go`, `static/js/src/app/templates.js`

### 3.2 OAuth 2.0 Authentication for SMTP Sending
- **Source:** Issue #3090, Issue #2026
- **Detail:** Google (May 2022) and Microsoft (Oct 2022) deprecated Basic Auth for SMTP/IMAP. Many operators can no longer use Gmail/Outlook SMTP.
- **Fix:**
  - Add `auth_type` field to SMTP model: `plain` | `login` | `oauth2`
  - For OAuth2: store `client_id`, `client_secret`, `refresh_token`, `token_url`
  - Use `golang.org/x/oauth2` to get access token before each send
  - UI: OAuth2 tab in Sending Profile modal with "Authorize" flow
- **Files:** `models/smtp.go`, `models/maillog.go`, `static/js/src/app/sending_profiles.js`, `templates/sending_profiles.html`

### 3.3 OAuth 2.0 for IMAP (Email Reporting)
- **Source:** Issue #2026
- **Same context as 3.2** — IMAP Basic Auth also deprecated by Google/Microsoft
- **Fix:** Add OAuth2 auth option to IMAP model alongside existing username/password
- **Files:** `models/imap.go`, `imap/imap.go`

### 3.4 Inline Image Embedding (CID Attachments)
- **Source:** Issue #251
- **Detail:** No native way to embed images inline (using `cid:` content-ID references) in email HTML without hosting them externally.
- **Fix:**
  - Allow image upload to template editor
  - Store base64-encoded images in template; encode as `multipart/related` with CID references in gomail
  - Add "Embed Image" button to CKEditor toolbar
- **Files:** `models/template.go`, `models/maillog.go`, `static/js/src/app/templates.js`

### 3.5 Extended Template Variables
- **Source:** PRs #9279, #1484; Issues #194, #1387, #1396
- **Fix (all in same file):**
  - `{{.Domain}}` — split `{{.Email}}` on `@`, take part after
  - `{{.FromEmail}}` — sender address from SMTP profile
  - `{{.Date}}` — formatted send date (configurable format string)
  - `{{.Time}}` — formatted send time
  - `{{.Company}}` — target's Company field (already stored in group targets, just not exposed)
  - `{{.Title}}` — target's Title/Position field
  - `{{.Phone}}` — target's Phone field
  - _(Note: Company/Title/Phone require zero DB changes — they are already stored on target rows)_
- **Files:** `models/template_context.go`

### 3.6 SMTPUTF8 Support (Homoglyph / Look-alike Attacks)
- **Source:** PR #2110
- **Fix:** Detect non-ASCII in sender address; enable `SMTPUTF8` extension via gomail dialer config; add `enable_smtputf8` toggle to SMTP model
- **Files:** `models/smtp.go`, `models/maillog.go`

### 3.7 Sending Profile Spoofed EHLO Hostname
- **Source:** PR #1400
- **Fix:** Add `smtp_hostname` to SMTP model; pass as `LocalName` to gomail dialer to override the real server hostname in EHLO/HELO
- **Files:** `models/smtp.go`, `models/maillog.go`

### 3.8 Per-Recipient Template Variation (A/B Testing)
- **Source:** Issue #3017
- **Overlap note:** Campaign Sets already handles coarse-grained variation — different templates per *group*. This item is finer-grained: random per-*individual* assignment within a single group (true A/B testing). The two are complementary, not redundant.
- **Detail:** Single campaign uses one template for all recipients. With weighted variants, each target randomly gets template A or B based on configured percentages.
- **Fix:**
  - Add `template_variants` JSON field to Campaign: `[{template_id, weight_percent}]`
  - Worker draws weighted random choice per target at send time
  - Results record `template_variant_id` so analysis can compare A vs B performance
- **Files:** `models/campaign.go`, `worker/worker.go`, migrations, `static/js/src/app/campaigns.js`

### 3.9 Attachment Template Support for HTA, PS1, BAT
- **Source:** PRs #2631, #2871
- **Fix:** Add `attachment_template_filetypes` to config.json; apply template variable rendering to matching MIME types beyond Office docs
- **Files:** `config/config.go`, `models/maillog.go`

### 3.10 Import Email Templates from .EML File
- **Source:** PR #1578
- **Fix:** File upload button → FileReader → API endpoint to parse EML → return extracted HTML + subject + attachments
- **Files:** `controllers/api/util.go`, `static/js/src/app/templates.js`, `templates/email_templates.html`

### 3.11 Credential Passthrough to Real Site
- **Source:** PR #1902, Issue #703
- **Fix:** After saving submitted credentials, server-side POST the same form data to a configurable `passthrough_url`; redirect victim to real site response so login appears successful
- **Files:** `models/page.go`, `controllers/phish.go`, `templates/landing_pages.html`, `static/js/src/app/landing_pages.js`

### 3.12 Descriptions for Campaigns, Templates, Pages
- **Source:** PR #1519
- **Fix:** Add `description TEXT` column to `campaigns`, `templates`, `pages` tables; optional textarea in each modal
- **Files:** `models/campaign.go`, `models/template.go`, `models/page.go`, migrations

### 3.13 CSS-Based Email Open Tracking (Outlook-Compatible)
- **Source:** Issue #1354
- **Detail:** Outlook Desktop (and some other clients) block `<img>` tracking pixels but load CSS `background-image` URLs. Operators targeting Outlook-heavy corporate environments get near-zero open events with the current pixel approach.
- **Fix:**
  - When "Add Tracking Image" is checked and template is rendered: additionally inject a 1-line `<style>` block with a unique `background-image: url(...)` pointing to the tracking endpoint
  - Tracking handler recognises both the `<img>` GET and the CSS GET as "Email Opened" events
  - Add `tracking_method` field to Template: `pixel` | `css` | `both` (default: `both`)
- **Files:** `models/template.go`, `models/maillog.go`, `controllers/phish.go`

### 3.14 Custom SMTP Headers per Sending Profile
- **Source:** Issue #2499
- **Detail:** Operators need to inject custom headers (e.g., `X-Phishing-Simulation: bypass`, `X-Mailer: CustomClient/1.0`) for internal allowlisting, mail-filter bypass whitelabels, or tracking correlation.
- **Fix:**
  - Add `custom_headers` JSON field to SMTP model: `[{"key": "X-Header", "value": "val"}]`
  - Apply headers to every outgoing email via gomail `SetHeader()`
  - UI: dynamic key/value row editor in Sending Profile modal
- **Files:** `models/smtp.go`, `models/maillog.go`, `static/js/src/app/sending_profiles.js`, migrations

---

## Phase 4 — Campaign Lifecycle Management

### 4.1 Campaign Pause / Resume
- **Source:** Issue #1342 (very popular, referenced in many issues)
- **Fix:**
  - Add `paused` status to campaign state machine
  - `POST /api/campaigns/{id}/pause` — stops worker from dispatching new emails
  - `POST /api/campaigns/{id}/resume` — restarts dispatch from where it left off
  - Worker checks `paused` flag before each send
  - UI: Pause/Resume button on campaign row and results page
- **Files:** `models/campaign.go`, `worker/worker.go`, `controllers/api/campaign.go`, `static/js/src/app/campaigns.js`

### 4.2 Campaign Edit via API
- **Source:** Issue #3276, Issue #3356
- **Fix:** Allow `PUT /api/campaigns/{id}` to update name, URL, template, SMTP profile for scheduled campaigns; running campaigns can only update template content, not group
- **Files:** `controllers/api/campaign.go`, `models/campaign.go`

### 4.3 Email Resend to Selected Targets
- **Source:** Issue #3356
- **Fix:**
  - `POST /api/campaigns/{id}/resend` with `{result_ids: [...]}`
  - Re-queues email sends for specific targets; useful for retry after bounce
  - UI: checkbox select on results table → "Resend" button
- **Files:** `controllers/api/campaign.go`, `worker/worker.go`, `static/js/src/app/campaign_results.js`

### 4.4 Remove Individual Target from Active Campaign
- **Source:** Issue #1931
- **Fix:**
  - `DELETE /api/campaigns/{id}/results/{rid}` — removes target from campaign metrics
  - Marks result as `excluded` in DB (not deleted, for audit)
  - UI: X button on each result row
- **Files:** `controllers/api/campaign.go`, `models/result.go`

### 4.5 Worker Auto-Complete Campaign at Scheduled End Time
- **Source:** PRs #1486, #1487
- **Partial status:** `send_by_date` already distributes emails to finish sending by a deadline (`generateSendDate()` in `campaign.go:307`). `CompletedDate` and `EndTime` fields exist in the model. What is **NOT done** is the worker actually checking `EndTime`/`CompletedDate` and automatically transitioning campaign status to `Completed` when that time passes.
- **Fix (narrow scope):**
  - In the worker background tick: if `campaign.EndTime` is set and `time.Now() > EndTime` → call `CompleteCampaign(id)`
  - Add `End Time` datetime picker to campaign modal (alongside existing `Send Emails By` field)
- **Files:** `worker/worker.go`, `static/js/src/app/campaigns.js`

### 4.6 Update Live Campaign Target Group
- **Source:** PR #3047
- **Fix:** On group save, check if active campaigns reference this group; if yes, generate `CampaignTarget` + queue email sends for newly added targets
- **Files:** `models/group.go`, `models/campaign.go`, `worker/worker.go`

### 4.7 Random RId Character Set and Length
- **Source:** PR #2162
- **Fix:** Add `rid_charset` and `rid_length` to Campaign model; use in `GenerateSecureKey()`; advanced options in campaign modal
- **Files:** `models/campaign.go`, `models/result.go`

### 4.8 Sandbox / Bot Click Filtering
- **Source:** Issue #2647, Issue #9389
- **Detail:** Email security tools pre-click all links to scan them, creating false positive click events. Additionally, Gmail proxies all images through Google's CDN (Issue #9389) — all email opens from Gmail appear to come from Google's IP ranges, making geo-tracking inaccurate and potentially flooding click events.
- **Fix:**
  - Configurable IP blocklist/allowlist for phish server (CIDR ranges)
  - Known sandbox UA string blocklist (configurable)
  - Pre-populate known bot CIDR ranges: Gmail Image Proxy (`66.102.0.0/20`, `209.85.128.0/17`), Microsoft SafeLinks (`40.94.0.0/16`), common security scanner ranges
  - Events from blocked IPs/UAs marked as `bot_event: true` and excluded from default stats
- **Files:** `controllers/phish.go`, `config/config.go`

---

## Phase 5 — Groups & Targets

### 5.1 Export Group Targets as CSV
- **Source:** PR #2233
- **Fix:** `GET /api/groups/{id}/csv` streams CSV; download button per group row
- **Files:** `controllers/api/user.go`, `static/js/src/app/groups.js`

### 5.2 Import Users from LDAP / Active Directory
- **Source:** PR #3308, Issues #28, #3158
- **Fix:** New `providers/ldap.go` package; `POST /api/import/ldap` endpoint; "Import from LDAP" modal in Groups page with server/bind DN/filter config
- **Files:** `providers/ldap.go` (new), `controllers/api/util.go`, `static/js/src/app/groups.js`

~~### 5.3 Add "Manager" Field to Targets~~ ❌ **CUT**
> Niche field with no query/reporting usage path in this fork. Results are reported by group/role, not org chart. Low ROI for the DB migration + UI effort. Removed.

~~### 5.4 Pagination for Large Groups (40k+ Targets)~~ ❌ **DUPLICATE — MERGED INTO 9.3**
> Fully covered by **9.3** (API Filtering + Server-Side Searching), which adds `?page=1&per_page=N` to **all** collection endpoints including groups. The groups-specific DataTables server-side mode should be implemented as the **first concrete target** when building 9.3. Source PRs #2019, #460, Issues #29, #415, #1971 noted there.

### 5.5 Random Subgroup Generator
- **Source:** PR #716
- **Fix:** `POST /api/groups/{id}/split` with `{count: N}`; creates N new groups; "Split into subgroups" button in groups UI
- **Files:** `controllers/api/user.go`, `models/group.go`, `static/js/src/app/groups.js`

### 5.6 Unicode Fix in CSV Import/Export
- **Source:** Issues #152, #154
- **Fix:** Set BOM / UTF-8 encoding header on CSV export; normalize UTF-8 on import before DB insert
- **Files:** `controllers/api/util.go`, `controllers/api/campaign.go`

### 5.7 Expand CSV Import Field Support
- **Source:** Issue #62
- **Fix:** Allow arbitrary extra columns in CSV; store as JSON in `target.extra_fields` column; expose as `{{.Custom.ColumnName}}` in templates
- **Files:** `models/group.go`, `models/template_context.go`, migrations

---

## Phase 6 — Results, Reporting & Analytics

### 6.1 Historical Snapshot + Range-Based Campaign Statistics
- **Source:** PR #9393
- **Fix:** `GET /api/campaigns/{id}/results/historical?mode=snapshot&cutoff=<ts>` and `mode=range&start=<ts>&end=<ts>`; results-page time-travel widget
- **Files:** `controllers/api/campaign.go`, `models/result.go`, `static/js/src/app/campaign_results.js`

### 6.2 Per-User / Per-Group Cross-Campaign Analytics
- **Source:** Issue #1966
- **Feature:** Organization-wide view: which users/groups clicked across all campaigns (repeat clickers, chronically vulnerable users).
- **Fix:**
  - `GET /api/results/aggregate?group_id=X` — returns merged results per unique email across all campaigns
  - New page `/analytics` with filterable table
- **Files:** `controllers/api/campaign.go` (new endpoint), `templates/analytics.html` (new), `static/js/src/app/analytics.js` (new)

### 6.3 Cross-Campaign Aggregate View
- **Source:** PR #728
- **Fix:** `GET /api/results/aggregate?campaign_ids=1,2,3`; new `/aggregate` page; table sorted by status
- **Files:** `controllers/api/campaign.go`, `templates/aggregate.html` (new), `static/js/src/app/aggregate.js` (new)

### 6.4 Mark Submitted Data as False Positive
- **Source:** PR #2208, Issue #1915
- **Fix:** Add `is_false_positive BOOL` to events table; `POST /api/campaigns/{id}/results/{rid}/false_positive`; toggle button next to submitted-data events; auto-demote status if all credential events are false-positives
- **Files:** `models/result.go`, `controllers/api/campaign.go`, `static/js/src/app/campaign_results.js`

### 6.5 Delete Captured Credentials
- **Source:** Issue #2227
- **Fix:** `DELETE /api/campaigns/{id}/results/{rid}/credentials` — removes credential data from `event.Details` while preserving the event metadata
- **Files:** `controllers/api/campaign.go`, `models/result.go`

### 6.6 Manual "Reported" Status + Custom Timestamp
- **Source:** Issue #3215
- **Fix:** `POST /api/campaigns/{id}/results/{rid}/report` with optional `reported_at` timestamp; "Mark as Reported" button in results UI
- **Files:** `controllers/api/campaign.go`, `static/js/src/app/campaign_results.js`

### 6.7 Table Load Settings (Filter Results Before Render)
- **Source:** PR #1125
- **Fix:** Status filter checkboxes above results table; `GET /api/campaigns/{id}/results?status=Clicked+Link,Submitted+Data`
- **Files:** `controllers/api/campaign.go`, `models/result.go`, `static/js/src/app/campaign_results.js`

### 6.8 Download Results as Formatted CSV
- **Source:** PR #2997
- **Fix:** `GET /api/campaigns/{id}/results.csv` streams formatted CSV with all fields
- **Files:** `controllers/api/campaign.go`

### 6.9 Add Sent Mail Details to Webhooks
- **Source:** PR #2493, Issue #1799
- **Fix:** Include subject, recipient, template name, SMTP profile in webhook payload for `email_sent` events; also add geo/demographic data (lat/long, position) to all event webhook payloads
- **Files:** `models/result.go`, `models/webhook.go`

### 6.10 Distinguish Attachment Open vs URL Click
- **Source:** Issue #3393
- **Fix:** Add `event_subtype` (url_click | attachment_open) to events; separate tracking URL pattern for attachments; different icon in results UI
- **Files:** `models/result.go`, `controllers/phish.go`, `static/js/src/app/campaign_results.js`

### 6.11 Full UA + IP Capture (all events, incl. Email Opened)
- **Source:** Issue #1920, PR #1148
- **Detail:** UA and IP are already stored for Link Clicked and Data Submitted events. Two gaps remain: (a) tracking pixel requests (Email Opened) don't capture UA at all; (b) PR #1148 identified that the results table/API doesn't surface full UA strings consistently across all event types.
- **Fix:**
  - Parse `User-Agent` header on tracking pixel request; store in Email Opened event details
  - Verify UA + IP are stored and exposed uniformly in all event types in the results API response
- **Files:** `controllers/phish.go`, `models/result.go`

### 6.12 Full Audit Log System
- **Source:** Issue #2189
- **Partial status:** Migration `20260101000000_activity_detection.sql` added `activity_detected BOOLEAN` to the `results` table — this is a single flag per result row, not a structured audit log. The full system (who created/edited/deleted what, and when) does **not** exist yet.
- **Fix:**
  - New `audit_log` table: `id, user_id, action (create|update|delete|launch|login), object_type, object_id, timestamp, details JSON`
  - Middleware hook: log every mutating API call (POST/PUT/DELETE) automatically
  - Log user logins and logouts explicitly
  - `GET /api/audit_log?page=1&per_page=50` endpoint with date range filters
  - Admin-only UI page to browse audit log
- **Files:** `models/audit.go` (new), `middleware/middleware.go`, `controllers/api/server.go`, migrations

---

## Phase 7 — UI/UX & Admin Features

### 7.1 OAuth 2.0 / OIDC SSO for Admin Login _(includes 7.3)_
- **Source:** PR #9363, Issues #3376, #3325, #2611
- **Fix:**
  - New `providers/oauth.go` with `golang.org/x/oauth2` + OIDC
  - Config: `sso_enabled`, `sso_provider`, `sso_client_id`, `sso_client_secret`, `sso_issuer_url`
  - `/oauth/callback` route; dynamic login page; optional SSO-only mode
  - `oauth_providers` table for multi-provider support
  - _(absorbed from 7.3)_ Add `disable_local_auth: true` to config.json; when set, `/login` returns 404 and forces SSO path — a 5-line config check in `controllers/admin.go`
- **Files:** `config/config.go`, `controllers/admin.go`, `providers/oauth.go` (new), migrations

### 7.2 Unauthenticated Health / Ping Endpoint
- **Source:** Issue #5877
- **Fix:** `GET /ping` (before auth middleware) returns `{"success": true, "version": "x.y.z"}` for K8s probes
- **Files:** `controllers/admin.go`

~~### 7.3 Disable Admin Login Page (for SSO-only Deployments)~~ ❌ **DUPLICATE — MERGED INTO 7.1**
> Sub-task of 7.1. `disable_local_auth` config flag is a 5-line addition that belongs inside the SSO implementation, not a standalone item.

### 7.4 Real-Time Activity Feed + Dashboard Timeline
- **Source:** Issue #196, Issue #1403
- **Fix:**
  - Server-Sent Events (SSE) endpoint `GET /api/events/stream` pushing campaign events in real-time; live event ticker widget on dashboard
  - Historical timeline chart on dashboard: events-per-hour bar chart across all active campaigns (Issue #1403); uses existing campaign results data, no new DB columns
- **Files:** `controllers/api/server.go` (new SSE handler), `static/js/src/app/dashboard.js`

### 7.5 User Feedback After Phishing Simulation
- **Source:** Issue #2684
- **Detail:** After user submits credentials or clicks link, show them it was a phishing simulation + optional training link.
- **Fix:**
  - Add `awareness_page_html` field to Page model
  - After credential capture, render awareness page instead of (or after) redirect
  - Configurable per-landing-page
- **Files:** `models/page.go`, `controllers/phish.go`, `templates/landing_pages.html`

### 7.6 Auto-Submit Landing Page After Countdown
- **Source:** Issue #2330
- **Fix:** Add `auto_submit_delay_seconds` to Page model; inject JavaScript countdown timer that auto-submits the form after N seconds (captures passive victims who open but don't interact)
- **Files:** `models/page.go`, `controllers/phish.go`

### 7.7 Multi-Page Landing Flow (Training Path)
- **Source:** Issue #127, Issue #219
- **Detail:** Single landing page → credentials → redirect. Operators want: page 1 credential harvest → page 2 training content → page 3 completion.
- **Fix:**
  - Add `next_page_id` foreign key to Page model for chaining
  - Phish server follows chain after credential submission
  - UI: "Next Page" selector in landing page modal
- **Files:** `models/page.go`, `controllers/phish.go`, `static/js/src/app/landing_pages.js`

~~### 7.8 Dynamic Redirect URL with Template Variables~~ ✅ **ALREADY DONE**
> `models/page.go:98` calls `ValidateTemplate(p.RedirectURL)` — the redirect URL is already treated as a Go template. `{{.Email}}`, `{{.RId}}`, `{{.FirstName}}` etc. work today. No action needed.

### 7.9 Tooltips on Group Target Table
- **Source:** PR #2089
- **Fix:** Truncate display to 15 chars + ellipsis; add `title` attribute with full value for Bootstrap tooltip on hover
- **Files:** `static/js/src/app/groups.js`

### 7.10 Localization / i18n _(Long-horizon only)_
- **Source:** PR #540, Issue #538
- **Scope warning:** All 21 JS modules and all HTML templates have hardcoded English strings with no abstraction layer. A proper i18n implementation would require touching every file. Estimate: 3–4 weeks of mechanical string extraction before any translation work begins.
- **Defer until:** All Phase 1–7 functional items are complete. Do not start this mid-sprint.
- **Fix (when ready):**
  - Add `language` to config.json
  - Create `translations/en.json` as baseline; add other locales incrementally
  - Write a thin JS i18n loader (`t('key')`) and do a single pass replacing strings
  - Language selector in Settings page
- **Files:** `config/config.go`, `translations/` (new), all `static/js/src/app/*.js`, all `templates/*.html`

### 7.11 User Last Activity Tracking
- **Source:** PR #3138, Issue #2327
- **Fix:** Add `last_activity TIMESTAMP` to users table; update via middleware (debounced, max once/min); expose in users API
- **Files:** `models/user.go`, `middleware/middleware.go`, migrations

### 7.12 Tooltips + Contextual Help on Dashboard _(Cosmetic — do last)_
- **Source:** Issues #90, #66
- **Note:** Pure UI polish with zero functional impact. Defer to absolute end of all other phases.
- **Fix:** Add `data-toggle="tooltip"` with explanatory text to dashboard stat cards; improve empty-state messaging for new users
- **Files:** `templates/dashboard.html`, `static/js/src/app/dashboard.js`

### 7.13 Extended / Nested Templates (Partials)
- **Source:** PR #1813, Issue #1779
- **Fix:** Custom `{{include "template_name"}}` directive; pre-process template HTML to resolve includes before rendering; "Include" picker in template editor
- **Files:** `models/template.go`, `models/maillog.go`

~~### 7.14 In-App Update Notification~~ ❌ **CUT**
> Upstream `gophish/gophish` has been dead since 2021 — there are no upstream releases to notify about. This fork diverges intentionally. Outbound GitHub API calls are undesirable in air-gapped or restricted deployments. No value. Removed.

---

## Phase 8 — Infrastructure & DevOps

### 8.1 Dependabot + CI/CD Automation
- **Source:** PR #2970, PR #2971, PR #2654, PR #2644
- **Fix:**
  - Add `.github/dependabot.yml` for Go + npm + Docker weekly checks
  - Add `.github/workflows/ci.yml` running `go test ./...` + `gofmt` on every PR
  - Add `.github/workflows/codeql.yml` for automated security scanning
  - Add `.github/workflows/docker.yml` to publish to `ghcr.io` on merge
- **Files:** `.github/` (new directory tree)

### 8.2 systemd Unit File _(Small but practical for bare-metal operators)_
- **Source:** PR #3285
- **Note:** While Docker covers containerized deployments, many red-team operators run GoPhish directly on a VPS without Docker. A unit file removes the need for manual `screen`/`tmux` setups.
- **Fix:** Single 15-line `contrib/gophish.service` file; add "Bare-Metal Service" section to README covering install path, env var setup, and `systemctl enable`
- **Files:** `contrib/gophish.service` (new), `README.md`

~~### 8.3 AWS Terraform Template~~ ❌ **CUT**
> Infrastructure provisioning is outside the tool's scope. Operators use their own IaC. Removed.

### 8.4 Kubernetes / Helm Deployment
- **Source:** PR #9332, Issue #2107
- **Fix:** Add `k8s/` with Deployment, Service, ConfigMap YAMLs; add `helm/gophish/` Helm chart; document persistent session key requirement
- **Files:** `k8s/` (new), `helm/` (new)

### 8.5 Working Directory CLI Flag
- **Source:** PR #3065
- **Fix:** Add `--workdir PATH` CLI flag via kingpin; `os.Chdir(workdir)` at startup before opening any files
- **Files:** `gophish.go`

### 8.6 PostgreSQL Support
- **Source:** Community PR #9396 (closed but worth implementing)
- **Fix:** Add `postgres` driver option alongside sqlite3/mysql; add `db/db_postgres/migrations/` directory; add `gorm.io/driver/postgres` to go.mod
- **Files:** `models/models.go`, `config/config.go`, `db/db_postgres/` (new), `go.mod`

---

## Phase 9 — Advanced / Long-Horizon Features

### 9.1 Custom Campaign Events
- **Source:** PR #1929
- **Fix:** `POST /api/arbevent/{rid}` for arbitrary events; configurable name, label, icon, color; events stored and visualized in results chart
- **Files:** `models/result.go`, `controllers/phish.go`, migrations, `static/js/src/app/campaign_results.js`

~~### 9.2 Per-Campaign Send Concurrency + Connection Pool~~ ❌ **DUPLICATE — REMOVED**
> Fully covered by **2.4** (Parallel/Concurrent Email Sending) and **2.10** (DB Connection Pool Configuration). Removed to avoid confusion.

### 9.3 API Filtering + Server-Side Searching _(includes merged 5.4)_
- **Source:** Issues #29, #33, #415, #1971; PRs #2019, #460
- **First implementation target:** Groups endpoint pagination (`GET /api/groups/{id}/targets?page=1&per_page=100`) and DataTables server-side mode — fixes browser hang on 40k+ imports. Then extend to all other collection endpoints.
- **Fix:** Standard `?q=search_term&filter[status]=active&page=1&per_page=50` query params on all collection endpoints; applied at DB layer not in-memory
- **Files:** All `controllers/api/*.go` that return collections, `models/group.go`, `static/js/src/app/groups.js`

~~### 9.4 SMTP Relay Hostname Override~~ ❌ **DUPLICATE — REMOVED**
> Identical to **3.7** (Sending Profile Spoofed EHLO Hostname). Removed.

~~### 9.5 Reporting Button for Outlook / G Suite (Add-in)~~ ❌ **CUT**
> Requires a separate Office.js/Workspace SDK project, enterprise IT add-in approval workflows, and distribution infrastructure. This is a standalone product, not a feature of this tool. Removed.

### 9.6 Generate RId Before Campaign Launch
- **Source:** Issue #3207
- **Fix:** Pre-generate all RIds when campaign is created; store in results table before any email sent; enables pre-tracking links inserted into documents/PDFs
- **Files:** `models/campaign.go`, `worker/worker.go`

### 9.7 Provide Raw SMTP Log per Sent Email
- **Source:** Issue #35
- **Fix:** Capture full SMTP conversation log (commands + responses) per email; store in `smtp_logs` table; expose via API for troubleshooting
- **Files:** `models/maillog.go`, `models/smtp_log.go` (new), migrations

---

## Known TODOs Already In Codebase

| File | Line | Status |
|------|------|--------|
| `controllers/api/campaign_set.go` | ~115 | ⚠️ PUT returns success but does NOT persist (`// TODO: Implement update`) |
| `models/scenario.go` | 18 | ✅ Fixed this session (backtick) |
| `static/js/src/app/utils.js` | 45,48 | ✅ Fixed this session (`group` → `user`) |
| `static/js/src/app/scenarios.js` | 181 | ✅ Fixed this session (`campaign` → `scenario`) |
| `static/js/src/app/gophish.js` | — | ✅ Fixed this session (`api.teamId` + `api.user` added) |

---

## Priority Matrix

| Phase | Title | Items | Priority | Effort | Notes |
|-------|-------|-------|----------|--------|-------|
| **1** | Security Fixes | 10 | 🔴 CRITICAL | Low–Med | 1.8 ✅ DONE; +1.9 (action strip), +1.10 (migration), +1.11 (From field) |
| **2** | Engine Reliability | 9 | 🔴 HIGH | Med–High | 2.2+2.3 ❌ merged, 2.6 ⚠️ isolated branch |
| **3** | Email Features | 14 | 🟠 HIGH | Med | 3.5 extended (+Company/Title/Phone); +3.13 CSS tracking, +3.14 custom headers |
| **4** | Campaign Lifecycle | 8 | 🟠 HIGH | Med | 4.5 ⚠️ EndTime tick only |
| **5** | Groups & Targets | 5 | 🟠 MEDIUM | Med | 5.3 ❌ CUT, 5.4 ❌ merged into 9.3 |
| **6** | Results & Reporting | 12 | 🟠 MEDIUM | Med | 6.12 ⚠️ full audit (boolean exists) |
| **7** | UI/UX & Admin | 11 | 🟡 MEDIUM | Low–Med | 7.3 ❌ merged into 7.1, 7.8 ✅ DONE, 7.14 ❌ CUT, 7.10/7.12 ⏳ defer last |
| **8** | Infrastructure | 5 | 🟡 LOW | Low | 8.3 ❌ CUT, 8.2 ⚠️ bare-metal only |
| **9** | Advanced/Long-horizon | 4 | 🟢 LOW | High | 9.2/9.4 ❌ dup removed, 9.5 ❌ CUT |

**Total actionable items: 78**
(Excludes 20 already done in this fork — 18 prior + 1.8 ca-certs + 7.8 dynamic redirect)
(3 merges: 2.2+2.3 → one item; 5.4 → into 9.3; 7.3 → into 7.1)
(+5 new from full GitHub audit: 1.9, 1.10, 1.11, 3.13, 3.14; 3.5 extended in-place)

---

## Suggested Execution Order

```
Sprint 1 (Week 1–2):   Phase 1 — 10 items (1.1–1.7 + new 1.9 action-strip bug, 1.10 migration, 1.11 From-field)
Sprint 2 (Week 3–4):   Phase 2.1–2.5 (event fix, rate+interval merged→2.2, parallel, random order)
Sprint 3 (Week 5–6):   Phase 3.1–3.5 (CC, OAuth SMTP, extended vars incl. Company/Title/Phone, inline images)
Sprint 4 (Week 7–8):   Phase 3.6–3.14 + Phase 4.1–4.4 (all email features incl. new CSS tracking + custom headers)
Sprint 5 (Week 9–10):  Phase 4.5–4.8 + Phase 5 (EndTime tick, remove target, subgroups; 5.3 cut + 5.4 merged → 5 items)
Sprint 6 (Week 11–12): Phase 6.1–6.6 (historical stats, cross-campaign analytics, false positive, delete creds)
Sprint 7 (Week 13–14): Phase 6.7–6.12 + Phase 7.1, 7.2 (remaining reporting + OAuth SSO incl. disable_local_auth, ping endpoint)
Sprint 8 (Week 15–16): Phase 7.4–7.7, 7.9, 7.11, 7.13 + Phase 8 (skip 7.3 merged / 7.8 done / 7.14 cut; 5 infra items, skip 8.3)
Sprint 9 (Week 17+):   Phase 2.6–2.7 (⚠️ isolated branch) + Phase 9.1, 9.3, 9.6, 9.7 (skip removed 9.2/9.4/9.5)
                        Phase 7.10 (i18n — 21 JS + all templates) only after all above complete
                        Phase 7.12 (dashboard tooltips) last — cosmetic
```

---

## Full Reference: PR/Issue → Fork Status

| # | Title | Type | Fork Status |
|----|-------|------|-------------|
| #9395 | Email sending failed | Issue | — support/bug (no feature) |
| #9394 | Send rate config for SMTP | PR | ❌ |
| #9393 | Historical campaign stats | PR | ❌ |
| #9392 | Event persistence fix | PR | ❌ |
| #9382 | GORM v2 + Goose v3 | PR | ❌ |
| #9381 | Go 1.22 + secure Docker | PR | ✅ (Go 1.24 + Docker done) |
| #9376 | CC email support | PR | ❌ |
| #9389 | Gmail Image Proxy blocks tracking pixel | Issue | ❌ noted in item 4.8 (pre-populated bot CIDR list) |
| #9368 | Session not invalidated on logout | Issue | ❌ |
| #9366 | API key not invalidated | Issue | ❌ |
| #9363 | OAuth 2.0 SSO | PR | ❌ |
| #9349 | User enumeration | Issue | ❌ |
| #9345 | Random send order | PR | ❌ |
| #9279 | {{.Domain}} variable | PR | ❌ |
| #5877 | /ping health endpoint | Issue | ❌ |
| #5810 | Dependency updates | PR | ✅ (Go 1.24 done) |
| #5809 | Send interval config | PR | ❌ merged into 2.2 (send rate + interval combined) |
| #3393 | Attachment open vs URL click | Issue | ❌ |
| #3378 | Multiple landing pages per campaign | Issue | ❌ see 7.7 (sequential chain: harvest → training → completion); A/B-style per-target page assignment covered by 3.8 |
| #3346 | QR code support | Issue | ✅ |
| #3338 | Landing page form action="" stripped on import | Issue | ❌ item 1.9 |
| #3325 | SSO integration | Issue | ❌ |
| #3316 | Parallel email sending | Issue | ❌ |
| #3310 | Dynamic redirect URL | Issue | ✅ (ValidateTemplate on redirect_url already in models/page.go:98) |
| #3308 | LDAP group import | PR | ❌ |
| #3278 | Race condition duplicate pages | Issue | ❌ |
| #3276 | Update campaign via API | Issue | ❌ |
| #3269 | Pre-auth SSRF | Issue | ❌ |
| #3247 | Recipient email as RId | Issue | ❌ NOT PLANNED — exposes PII in URLs/logs; use item 4.7 (custom RId charset/length) for correlation needs instead |
| #3234 | Future maintainership | Issue | ✅ (we are the fork) |
| #3215 | Manual Reported + timestamp | Issue | ❌ |
| #3207 | Pre-generate RId | Issue | ❌ |
| #3199 | Outlook/Gmail report add-in | Issue | ❌ CUT (standalone project, out of scope) |
| #3138 | User last activity field | PR | ❌ |
| #3090 | OAuth SMTP (Gmail/M365) | Issue | ❌ |
| #3065 | Working directory CLI flag | PR | ❌ |
| #3063 | Pure-Go SQLite (no CGO) | PR | ❌ |
| #3047 | Update live campaign group | PR | ❌ |
| #2997 | Download results CSV | PR | ❌ |
| #2991 | XSS on campaign delete | PR | ❌ |
| #2897 | QR code in emails | PR | ✅ |
| #2889 | Docker missing ca-certs | Issue | ✅ (ca-certificates already in Dockerfile runtime stage) |
| #2864 | API key security fix | PR | ❌ |
| #2819 | Docker DB directory | PR | ✅ |
| #2684 | User feedback after simulation | Issue | ❌ |
| #2651 | HTML redirect after submit | PR | ✅ (redirect_html) |
| #2647 | Sandbox click filtering | Issue | ❌ |
| #2612 | Multi-user collaboration | Issue | ✅ (Teams + RBAC done) |
| #2611 | Disable admin login page | Issue | ❌ merged into 7.1 (SSO item, disable_local_auth flag) |
| #2529 | DB connection pool config | Issue | ❌ |
| #2499 | Custom SMTP headers per Sending Profile | Issue | ❌ item 3.14 |
| #2493 | Sent details in webhooks | PR | ❌ |
| #2487 | Restart fail if admin deleted | Issue | ❌ item 1.7 |
| #2481 | account_locked column missing on migrated DBs | Issue | ❌ item 1.10 |
| #2443 | Angle brackets broken in SMTP From field | Issue | ❌ item 1.11 |
| #2424 | Manager field for targets | PR | ❌ CUT (niche, not worth schema churn) |
| #2330 | Auto-submit after countdown | Issue | ❌ |
| #2327 | User last login tracking | Issue | ❌ |
| #2233 | Export group CSV | PR | ❌ |
| #2227 | Delete captured credentials | Issue | ❌ |
| #2213 | API key session security | PR | ❌ |
| #2208 | Mark as false positive | PR | ❌ |
| #2189 | Verbose audit logs | Issue | ❌ |
| #2162 | Custom RId charset/length | PR | ❌ |
| #2110 | SMTPUTF8 homoglyph support | PR | ❌ |
| #2107 | K8s CSRF pod restart | Issue | ❌ |
| #2089 | Tooltips on target table | PR | ❌ ⏳ deferred last (cosmetic, 7.12) |
| #2026 | OAuth IMAP (M365/Gmail) | Issue | ❌ |
| #2019 | Pagination for large groups | PR | ❌ merged into 9.3 (first target of API filtering implementation) |
| #1966 | Per-user/group analytics | Issue | ❌ |
| #1931 | Remove target from campaign | Issue | ❌ |
| #1929 | Custom campaign events | PR | ❌ |
| #1920 | UA on Email Opened events | Issue | ❌ |
| #1894 | Non-campaign email reports | PR | ✅ (IMAP monitor) |
| #1813 | Extended/nested templates | PR | ❌ |
| #1799 | Webhooks include full details | Issue | ❌ |
| #1578 | Import EML file | PR | ❌ |
| #1557 | Send frequency config | PR | ❌ merged into 2.2 (send rate + interval combined) |
| #1519 | Descriptions on objects | PR | ❌ |
| #1487 | Schedule campaign completion | PR | ❌ see item 4.5 (also cites PR #1486 — same feature) |
| #1484 | {{.FromEmail}} variable | PR | ❌ |
| #1400 | SMTP EHLO hostname override | PR | ❌ |
| #1403 | Dashboard Timeline visualization | Issue | ❌ merged into item 7.4 (historical chart + live feed) |
| #1396 | {{.Company}}, {{.Title}}, {{.Phone}} template vars | Issue | ❌ extended into item 3.5 |
| #1387 | {{.Date}} variable | Issue | ❌ |
| #1354 | CSS open tracking for email clients (Outlook) | Issue | ❌ item 3.13 |
| #1342 | Campaign pause/resume | Issue | ❌ |
| #1154 | AWS Terraform template | PR | ❌ CUT (out of scope — infra managed outside app) |
| #1148 | Full UA + IP in results | PR | ❌ see item 6.11 (expanded to cover all events + tracking pixel) |
| #1125 | Table load settings | PR | ❌ |
| #728 | Aggregate results all campaigns | PR | ❌ |
| #716 | Random subgroup generator | PR | ❌ |
| #540 | Localization / i18n | PR | ❌ ⏳ deferred last (21 JS files + all templates, 7.10) |
| #460 | Pagination in groups page | PR | ❌ merged into 9.3 (first implementation target) |
| #251 | Inline image embedding | Issue | ❌ |
| #219 | Multiple redirect/landing pages | Issue | ❌ |
| #196 | Real-time activity feed | Issue | ❌ |
| #154 | Unicode fix CSV export | Issue | ❌ |
| #152 | Unicode fix CSV import | Issue | ❌ |
| #129 | IMAP for bounce/reply tracking | Issue | ✅ (IMAP monitor done) |
| #127 | Multi-page training flow | Issue | ❌ |
| #104 | In-app update notification | Issue | ❌ CUT (upstream dead — no release API to check against) |
| #62 | Custom CSV import fields | Issue | ❌ |
| #33 | API filtering/searching | Issue | ❌ |
| #29 | API pagination | Issue | ❌ |
| #28 | LDAP import | Issue | ❌ |
| #27 | Custom template variables | Issue | ✅ (many added) |
