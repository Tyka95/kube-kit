# P1 — AWS Session State & DB Discovery

**Status:** Draft for review
**Scope:** Phase 1 of three-phase KubeKit v0.2 work. See follow-ups for P2 (consistent picker) and P3 (visual identity).
**Owner:** KubeKit
**Date:** 2026-05-11

## Problem

KubeKit's auth model is reactive at startup and never updates:

- `_CTX_AWS_PROFILE` is sampled once from `$AWS_PROFILE`/kubeconfig and shown as a static footer label. It does not reflect what AWS calls will actually use.
- The footer shows `☁ stage` even when the kubectl context targets a production cluster owned by a different AWS account — silently misleading.
- DB discovery in `lib/db_forward.sh` calls `aws rds describe-*` with `2>/dev/null || result=""`. Expired SSO credentials produce an empty list with no error.
- The same path writes an empty result to a 5-minute on-disk cache (`~/.local/state/kubekit/db_cache_<profile>_<region>`), so re-running after `aws sso login` still shows nothing until the cache expires.
- There is no UI affordance to refresh discovery, switch profile, or surface an auth failure.

The combined effect: users routinely see an empty or wrong DB list, close the terminal, run `aws sso login` or `assume` manually, and try again. The tool gives no signal explaining why.

## Goals

1. A single, observable, refreshable AWS session state owned by KubeKit.
2. Footer reflects real auth status (validated, expired, account mismatch) — not just a label.
3. DB discovery either succeeds, asks the user to authenticate, or shows a clear error. It never silently returns an empty list.
4. No cache can survive an auth state change. Failures never poison future runs.
5. The same recovery pattern (`aws sso login`) is reused everywhere AWS is called.

## Non-goals

- Multi-account fan-out (discovering across multiple profiles in one screen). Single active profile only; users switch profiles to see other accounts.
- Modelling STS assume-role chains. The AWS CLI handles `source_profile` / `role_arn` transparently.
- Watching `~/.aws/credentials` for changes between menu entries. Validation on menu entry plus a short TTL is enough.
- Any visual redesign — Phase 3 owns that.

## Design

### Component 1 — `lib/aws_session.sh` (new)

Single source of truth for AWS identity. In-memory only.

State:

```
AWS_SESSION_PROFILE       resolved profile name (string, may be empty)
AWS_SESSION_REGION        resolved region (string, may be empty)
AWS_SESSION_ACCOUNT       account id from sts get-caller-identity (12 digits, or empty)
AWS_SESSION_ARN           caller arn (or empty)
AWS_SESSION_STATUS        ok | expired | unknown | no-aws
AWS_SESSION_ERROR         last error message, one line, for surfacing in UI
AWS_SESSION_CHECKED_AT    epoch seconds of last successful sts call
```

Public functions:

| Function | Behavior |
|----------|----------|
| `aws_session_resolve` | Recomputes `PROFILE` and `REGION` from kubectl exec env → `$AWS_PROFILE` → `$AWS_DEFAULT_PROFILE` → `CFG_DEFAULT_PROFILE`, and EKS context ARN → `$AWS_REGION` → `$AWS_DEFAULT_REGION` → `CFG_AWS_REGIONS[0]`. No hard-coded defaults. |
| `aws_session_validate [force]` | Runs `aws sts get-caller-identity --output text --query 'Account,Arn'` with `--cli-connect-timeout 3 --cli-read-timeout 5`. Updates `ACCOUNT`, `ARN`, `STATUS`, `ERROR`, `CHECKED_AT`. Skips work if `CHECKED_AT` is within 60 s unless `force=1`. |
| `aws_session_login` | Runs `aws sso login --profile <PROFILE>` interactively via `run_interactive`. Re-validates with `force=1` afterwards. Returns 0 if `STATUS=ok`. |
| `aws_session_ensure` | Validates; if `STATUS=expired`, prompts the user (`gum confirm`) to run `aws_session_login`; on success, returns 0. Returns 1 if the user declines or login still fails. Used as a gate at the start of every AWS-using action. |
| `aws_session_context_changed` | Called when kubectl context or `$AWS_PROFILE` changes. Re-runs `resolve` and clears `ACCOUNT`/`ARN`/`STATUS` so the next `validate` is forced. Also clears `DB_CACHE_*` (see Component 3). |

`STATUS` transitions:

```
unknown ──validate──► ok
unknown ──validate──► expired      (sts returns ExpiredToken / InvalidClientTokenId)
unknown ──validate──► no-aws       (aws cli missing, or no profile + no env)
ok      ──60s TTL──► (no-op; revalidate on next ensure)
ok      ──context_changed──► unknown
expired ──login──► ok | expired
```

### Component 2 — `_footer_bar` redesign in `lib/ui.sh`

Replace the current static `☁ <profile>` segment with a real status segment:

```
⎈ production-eks  │  ⬡ default  │  ☁ <profile> <glyph> <detail>  ⏱ <ttl>
```

Glyph + detail mapping:

| `AWS_SESSION_STATUS` | Glyph | Detail |
|----------------------|-------|--------|
| `unknown` (validating) | `…` (dim) | `validating` |
| `ok` (no mismatch) | `✓` (green) | account id (12 digits) |
| `ok` + mismatch (see below) | `⚠` (yellow) | `mismatch ⟶ <ctx-account>` |
| `expired` | `✗` (red) | `expired — press R to refresh` |
| `no-aws` | `–` (dim) | `no aws` |

Mismatch detection: if the kubectl current-context is an EKS ARN, extract its account id (`arn:aws:eks:<region>:<account>:cluster/...`) and compare against `AWS_SESSION_ACCOUNT`. Inequality → mismatch state. This catches the documented `production-eks` + `stage`-profile scenario directly.

Footer redraws via the existing `_redraw_footer` path. Validation is triggered:

- on startup (background, non-blocking; footer initially shows `… validating`)
- on kubectl context switch (`actions_cluster.sh`)
- on AWS profile switch (new `P` keybinding in P2; for P1 we add `aws_session_context_changed` to existing profile-changing flows)
- on entering DB / S3 / AWS submenus (via `aws_session_ensure`)
- on the existing `_tick` 150-tick footer refresh, if `CHECKED_AT` is older than 60 s

### Component 3 — DB cache redesign in `lib/db_forward.sh`

Replace on-disk cache with in-memory state, keyed by *validated identity*:

```
DB_CACHE_ACCOUNT=""     # account id at time of fetch
DB_CACHE_REGION=""      # region at time of fetch
DB_CACHE_ENTRIES=()     # array of "Name|host|port"
DB_CACHE_TS=0           # epoch
DB_CACHE_ERROR=""       # non-empty if last attempt failed
```

Rules:

1. Cache is valid iff `DB_CACHE_ACCOUNT == AWS_SESSION_ACCOUNT && DB_CACHE_REGION == AWS_SESSION_REGION && now - DB_CACHE_TS < 60`.
2. Any auth state change (`aws_session_context_changed`) zeros all `DB_CACHE_*` variables.
3. On `aws rds describe-*` failure, `DB_CACHE_ENTRIES` stays empty and `DB_CACHE_ERROR` captures the first line of stderr. `DB_CACHE_TS` is **not** updated, so the next attempt retries immediately.
4. No disk file is written. The directory `~/.local/state/kubekit/db_cache_*` is no longer used; existing files are cleaned up on first run by `aws_session.sh` init.

### Component 4 — DB picker behavior

`db_forward()` flow:

1. Render header.
2. Call `aws_session_ensure`. If it returns non-zero (user declined login, or login failed), render an inline message:
   ```
   AWS session not available for profile <P>. Discovery skipped.
   Press R to retry, or pick a configured target / Custom endpoint.
   ```
3. If `STATUS=ok`, run `_discover_db_targets` only if cache is invalid. Show a brief `Discovering…` line in the content area; clear it after.
4. Build options list: **configured `db_target`s → discovered entries (deduped by hostname) → Custom endpoint**. If `DB_CACHE_ERROR` is set, prepend a dim line: `discovery error: <message>`.
5. Picker. Until P2 lands we keep `gum choose`, but consume the standard return codes so refresh/profile keybindings can be added in P2 without rework.

### Component 5 — Wire-up

- `kubekit.sh`: source `lib/aws_session.sh` before `lib/ui.sh` (footer needs it).
- `lib/context.sh`: keep `_CTX_AWS_PROFILE` for backwards compatibility but treat `AWS_SESSION_*` as authoritative.
- `lib/session.sh` (`refresh_aws_sso` for kubectl): becomes a thin call to `aws_session_login`.
- `lib/actions_aws.sh`: each AWS action's entry calls `aws_session_ensure` and returns early on failure.
- On startup `main()`: schedule one `aws_session_validate` in the background so the footer becomes accurate within the first second without blocking the menu.

## Error handling

| Failure mode | Behavior |
|---|---|
| `aws` not installed | `STATUS=no-aws`, footer shows `–`, AWS submenus disabled with explanatory line. |
| Profile unset and no `$AWS_PROFILE` | `STATUS=no-aws`. DB picker still works (configured targets + Custom). |
| `sts` returns `ExpiredToken` / `InvalidClientTokenId` | `STATUS=expired`. `aws_session_ensure` prompts to login. |
| `sts` times out | `STATUS=unknown`, `ERROR="sts timed out"`. Footer shows `?` glyph. |
| `aws rds describe-*` fails after `STATUS=ok` | `DB_CACHE_ERROR` set, picker shows dim error line; discovery retried on next entry. |
| Mismatch (context account ≠ profile account) | `STATUS=ok` but UI shows `⚠ mismatch ⟶ <ctx-account>`. Discovery still runs against the resolved profile; results carry an `[<profile>]` suffix so users see which account they came from. |

## Backwards compatibility

- `~/.config/kubekit/config` keys (`aws_regions`, `default_namespace`, `default_profile`, `db_target`) remain supported.
- The on-disk cache directory `~/.local/state/kubekit/db_cache_*` is no longer read or written; existing files are silently deleted on first run.
- Footer string format changes (new glyphs/segments). No external consumers parse it, so this is fine.

## Testing

Manual scenarios that must pass before P1 ships:

1. **Happy path:** valid SSO session, context = `production-eks`, profile resolves to a prod profile → footer shows `✓ <account>`, DB picker lists configured + discovered prod entries.
2. **Mismatch:** context = `production-eks`, profile = `stage` → footer shows `⚠ mismatch ⟶ 963758802620`, discovery runs against stage and shows stage results tagged `[stage]`.
3. **Expired token:** revoke SSO, enter DB menu → `gum confirm` to login → after login, picker re-discovers automatically. No cached emptiness shown.
4. **No aws cli:** discovery silently skipped, config + Custom endpoint still work, footer shows `–`.
5. **Profile switch mid-session** (via existing `pick_aws_profile` flow): footer updates, cache cleared, next discovery hits the new account.
6. **Cache TTL:** within 60 s of a successful discovery, second entry into picker does not hit AWS (verify via `set -x` or count).
7. **Cache cleanup:** any pre-existing `~/.local/state/kubekit/db_cache_*` file is removed after first run.

No automated tests in this codebase yet; out of scope for P1.

## Risks

- `gum confirm` for SSO login interrupts flow on every expiry. Mitigated by 60 s TTL — the prompt only appears at most once per minute per session.
- Background validation on startup uses a subshell and writes back via shared file (or named pipe) since bash variables don't cross subshells. Implementation will use a temp file under `$XDG_RUNTIME_DIR` or `mktemp`, read by `_tick` on its next pass.
- `sts get-caller-identity` is ~150 ms typical; cost is acceptable, and the timeout caps worst case at 5 s.

## Out of scope, deferred to later phases

- **P2:** Replace `gum choose` with KubeKit's native `choose_menu` everywhere so `R` (refresh) / `P` (switch profile) keybindings work uniformly and animations don't freeze during pickers.
- **P3:** Header redesign (drop morphing icon), tighter palette, consistent spacing, references from k9s / lazygit / gh dash.
