# P1 — AWS Session State & DB Discovery — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace KubeKit's static AWS profile label with a live, validated session model; surface auth state in the footer; make DB discovery either succeed, prompt for `aws sso login`, or show a clear error — never silently empty.

**Architecture:** A new `lib/aws_session.sh` module owns AWS identity state. Every AWS-using action calls `aws_session_ensure` as a gate. The footer reads from this state and shows mismatch / expired / validating glyphs. DB discovery caches results in memory keyed by `(account_id, region)` so auth changes invalidate the cache automatically.

**Tech Stack:** bash 3.2+, AWS CLI v2, `gum` (already a hard dep), `kubectl`. No automated test framework — verification is manual via `bash -n`, manual scenario walkthroughs, and `set -x` traces where useful.

**Testing note:** This codebase has no test runner. Each task ends with a manual verification step describing the exact commands and expected on-screen behavior. Commits happen after each verification passes.

**Working directory:** `/Users/constantinsurdu/Repos/Personal/kube-kit`

**Reference:** [P1 spec](../specs/2026-05-11-p1-aws-session-and-discovery-design.md)

---

## File Structure

**Created:**
- `lib/aws_session.sh` — session state + resolve / validate / login / ensure / context_changed

**Modified:**
- `kubekit.sh` — source aws_session.sh
- `lib/ui.sh` — footer reads `AWS_SESSION_*`, shows status glyphs
- `lib/animation.sh` — `_tick` periodic revalidation
- `lib/db_forward.sh` — drop disk cache, in-memory only, gate via `aws_session_ensure`
- `lib/session.sh` — `refresh_aws_sso` delegates to `aws_session_login`
- `lib/actions_aws.sh` — each AWS action calls `aws_session_ensure`

**Deleted at runtime (not in repo):**
- `~/.local/state/kubekit/db_cache_*` — cleaned by aws_session.sh on first load

---

## Task 1: Create `lib/aws_session.sh` skeleton with state and `aws_session_resolve`

**Files:**
- Create: `lib/aws_session.sh`

- [ ] **Step 1: Create the file with state variables and `aws_session_resolve`**

```bash
# lib/aws_session.sh
# ── AWS session state ────────────────────────────────────────────────────────
# Single source of truth for AWS identity used by every AWS-aware action.

AWS_SESSION_PROFILE=""
AWS_SESSION_REGION=""
AWS_SESSION_ACCOUNT=""
AWS_SESSION_ARN=""
AWS_SESSION_STATUS="unknown"   # unknown | ok | expired | no-aws
AWS_SESSION_ERROR=""
AWS_SESSION_CHECKED_AT=0

# Resolve PROFILE and REGION from kubeconfig exec env > env vars > kubekit config.
# No hardcoded defaults — empty result means "unknown".
aws_session_resolve() {
  local profile=""
  profile=$(kubectl config view --minify -o jsonpath='{.users[0].user.exec.env[?(@.name=="AWS_PROFILE")].value}' 2>/dev/null) || true
  [[ -z "$profile" ]] && profile="${AWS_PROFILE:-${AWS_DEFAULT_PROFILE:-${CFG_DEFAULT_PROFILE:-}}}"
  AWS_SESSION_PROFILE="$profile"

  local region=""
  local ctx
  ctx=$(kubectl config current-context 2>/dev/null) || true
  if [[ "$ctx" =~ arn:aws:eks:([a-z0-9-]+): ]]; then
    region="${BASH_REMATCH[1]}"
  fi
  [[ -z "$region" ]] && region="${AWS_REGION:-${AWS_DEFAULT_REGION:-}}"
  if [[ -z "$region" && ${#CFG_AWS_REGIONS[@]} -gt 0 ]]; then
    region="${CFG_AWS_REGIONS[0]}"
  fi
  AWS_SESSION_REGION="$region"
}

# Return the account id encoded in the current kubectl context's EKS ARN, if any.
aws_session_context_account() {
  local ctx
  ctx=$(kubectl config current-context 2>/dev/null) || true
  if [[ "$ctx" =~ arn:aws:eks:[a-z0-9-]+:([0-9]{12}): ]]; then
    printf '%s' "${BASH_REMATCH[1]}"
  fi
}
```

- [ ] **Step 2: Syntax check**

Run: `bash -n /Users/constantinsurdu/Repos/Personal/kube-kit/lib/aws_session.sh`
Expected: no output, exit 0.

- [ ] **Step 3: Smoke test the resolve function**

Run from the repo root:
```bash
( source lib/config.sh; source lib/aws_session.sh; aws_session_resolve; \
  echo "PROFILE=$AWS_SESSION_PROFILE REGION=$AWS_SESSION_REGION" )
```
Expected: prints a `PROFILE=...` and `REGION=...` line. Profile/region may be empty if shell has no AWS env — that's correct.

- [ ] **Step 4: Commit**

```bash
git add lib/aws_session.sh
git commit -m "feat(aws-session): scaffold session module with state + resolve"
```

---

## Task 2: Add `aws_session_validate`

**Files:**
- Modify: `lib/aws_session.sh` (append)

- [ ] **Step 1: Append the validate function**

Add to the end of `lib/aws_session.sh`:

```bash
# Validate the resolved session via sts get-caller-identity.
# Updates ACCOUNT, ARN, STATUS, ERROR, CHECKED_AT.
# Skips work if last successful check was <60s ago, unless $1=1 (force).
aws_session_validate() {
  local force="${1:-0}"
  local now
  now=$(date +%s)

  if [[ "$force" != "1" && "$AWS_SESSION_STATUS" == "ok" ]]; then
    if (( now - AWS_SESSION_CHECKED_AT < 60 )); then
      return 0
    fi
  fi

  aws_session_resolve

  if ! command -v aws &>/dev/null; then
    AWS_SESSION_STATUS="no-aws"
    AWS_SESSION_ACCOUNT=""
    AWS_SESSION_ARN=""
    AWS_SESSION_ERROR="aws cli not installed"
    return 1
  fi

  local args=()
  [[ -n "$AWS_SESSION_PROFILE" ]] && args+=(--profile "$AWS_SESSION_PROFILE")
  [[ -n "$AWS_SESSION_REGION" ]] && args+=(--region "$AWS_SESSION_REGION")
  args+=(--cli-connect-timeout 3 --cli-read-timeout 5 --output text --query 'Account,Arn')

  local out err rc=0
  err=$(mktemp)
  out=$(aws sts get-caller-identity "${args[@]}" 2>"$err") || rc=$?

  if (( rc == 0 )) && [[ -n "$out" ]]; then
    AWS_SESSION_ACCOUNT="${out%%[[:space:]]*}"
    AWS_SESSION_ARN="${out#*[[:space:]]}"
    AWS_SESSION_STATUS="ok"
    AWS_SESSION_ERROR=""
    AWS_SESSION_CHECKED_AT="$now"
    rm -f "$err"
    return 0
  fi

  local err_line
  err_line=$(head -n1 "$err" 2>/dev/null)
  rm -f "$err"
  AWS_SESSION_ACCOUNT=""
  AWS_SESSION_ARN=""
  AWS_SESSION_ERROR="${err_line:-sts call failed}"
  if [[ "$err_line" == *"ExpiredToken"* || "$err_line" == *"InvalidClientTokenId"* || \
        "$err_line" == *"Token has expired"* || "$err_line" == *"SSO session"* ]]; then
    AWS_SESSION_STATUS="expired"
  elif [[ -z "$AWS_SESSION_PROFILE" && -z "${AWS_ACCESS_KEY_ID:-}" ]]; then
    AWS_SESSION_STATUS="no-aws"
  else
    AWS_SESSION_STATUS="expired"
  fi
  return 1
}
```

- [ ] **Step 2: Syntax check**

Run: `bash -n lib/aws_session.sh`
Expected: no output, exit 0.

- [ ] **Step 3: Manual verification — happy path**

With a valid SSO session loaded in the current shell:
```bash
( source lib/config.sh; source lib/aws_session.sh; \
  aws_session_validate 1; \
  echo "STATUS=$AWS_SESSION_STATUS ACCOUNT=$AWS_SESSION_ACCOUNT" )
```
Expected: `STATUS=ok ACCOUNT=<12-digit-id>`.

- [ ] **Step 4: Manual verification — expired path**

Unset credentials and run again:
```bash
( unset AWS_PROFILE AWS_ACCESS_KEY_ID; \
  source lib/config.sh; source lib/aws_session.sh; \
  aws_session_validate 1; \
  echo "STATUS=$AWS_SESSION_STATUS ERROR=$AWS_SESSION_ERROR" )
```
Expected: `STATUS=no-aws` (no profile set) **or** `STATUS=expired` (profile set but no creds), with a non-empty `ERROR=`.

- [ ] **Step 5: Commit**

```bash
git add lib/aws_session.sh
git commit -m "feat(aws-session): add validate via sts get-caller-identity"
```

---

## Task 3: Add `aws_session_login` and `aws_session_ensure`

**Files:**
- Modify: `lib/aws_session.sh` (append)

- [ ] **Step 1: Append login and ensure functions**

Add to the end of `lib/aws_session.sh`:

```bash
# Interactively run `aws sso login --profile <P>` and re-validate.
# Returns 0 if STATUS=ok after login, 1 otherwise.
aws_session_login() {
  if [[ -z "$AWS_SESSION_PROFILE" ]]; then
    AWS_SESSION_ERROR="no profile to log in with"
    return 1
  fi
  if ! command -v aws &>/dev/null; then
    AWS_SESSION_STATUS="no-aws"
    AWS_SESSION_ERROR="aws cli not installed"
    return 1
  fi

  # run_interactive is defined in lib/session.sh; if absent, fall back to direct call.
  if declare -F run_interactive &>/dev/null; then
    run_interactive aws sso login --profile "$AWS_SESSION_PROFILE" || true
  else
    aws sso login --profile "$AWS_SESSION_PROFILE" >&3 2>&3 || true
  fi
  aws_session_validate 1
}

# Gate for every AWS-using action.
# - STATUS=ok      → return 0 immediately (or after a fresh validate if TTL expired).
# - STATUS=expired → prompt the user (gum confirm) to run `aws sso login`; retry once.
# - STATUS=no-aws  → return 1 silently; caller is expected to skip discovery gracefully.
aws_session_ensure() {
  aws_session_validate
  case "$AWS_SESSION_STATUS" in
    ok)     return 0 ;;
    no-aws) return 1 ;;
    expired)
      local prompt="AWS session expired for profile '${AWS_SESSION_PROFILE:-<none>}'. Run 'aws sso login' now?"
      if command -v gum &>/dev/null && gum confirm "$prompt"; then
        aws_session_login && return 0
      fi
      return 1
      ;;
    *) return 1 ;;
  esac
}

# Call when kubectl context or AWS profile changes — forces re-resolve and invalidates
# any downstream caches keyed on the previous identity.
aws_session_context_changed() {
  AWS_SESSION_STATUS="unknown"
  AWS_SESSION_ACCOUNT=""
  AWS_SESSION_ARN=""
  AWS_SESSION_CHECKED_AT=0
  aws_session_resolve
  # Clear in-memory DB cache (variables defined in db_forward.sh).
  DB_CACHE_ACCOUNT=""
  DB_CACHE_REGION=""
  DB_CACHE_ENTRIES=()
  DB_CACHE_TS=0
  DB_CACHE_ERROR=""
}

# One-time cleanup of stale on-disk cache from pre-P1 versions.
_aws_session_cleanup_legacy() {
  local d="${HOME}/.local/state/kubekit"
  [[ -d "$d" ]] || return 0
  rm -f "$d"/db_cache_* 2>/dev/null || true
}
_aws_session_cleanup_legacy
```

- [ ] **Step 2: Syntax check**

Run: `bash -n lib/aws_session.sh`
Expected: no output, exit 0.

- [ ] **Step 3: Verify legacy cleanup runs**

```bash
mkdir -p ~/.local/state/kubekit && touch ~/.local/state/kubekit/db_cache_test
( source lib/config.sh; source lib/aws_session.sh )
ls ~/.local/state/kubekit/db_cache_* 2>&1
```
Expected last line: `ls: no matches found:` (or equivalent — file is gone).

- [ ] **Step 4: Commit**

```bash
git add lib/aws_session.sh
git commit -m "feat(aws-session): add login, ensure, context_changed + legacy cleanup"
```

---

## Task 4: Wire `aws_session.sh` into `kubekit.sh`

**Files:**
- Modify: `kubekit.sh`

- [ ] **Step 1: Add source line after `config.sh`**

Find the block in `kubekit.sh`:
```bash
source "$KUBE_LIB/config.sh"
source "$KUBE_LIB/theme.sh"
```
Insert immediately after `config.sh`:
```bash
source "$KUBE_LIB/aws_session.sh"
```
Result:
```bash
source "$KUBE_LIB/config.sh"
source "$KUBE_LIB/aws_session.sh"
source "$KUBE_LIB/theme.sh"
```

- [ ] **Step 2: Syntax check the whole launcher**

Run: `bash -n kubekit.sh`
Expected: no output, exit 0.

- [ ] **Step 3: Smoke-launch**

Run: `./kubekit.sh --version`
Expected: `kubekit 0.1.8`.

- [ ] **Step 4: Commit**

```bash
git add kubekit.sh
git commit -m "feat(aws-session): source aws_session.sh in launcher"
```

---

## Task 5: Footer reads `AWS_SESSION_*` and shows status glyphs

**Files:**
- Modify: `lib/ui.sh` (function `_footer_bar`, currently ~line 37-58)

- [ ] **Step 1: Replace the AWS segment of `_footer_bar`**

Find this block in `lib/ui.sh`:
```bash
  if [[ -n "$_CTX_AWS_PROFILE" ]]; then
    _ftr+="  ${C_DIM}│${C_RESET}  ${C_CYAN}☁${C_RESET} ${_CTX_AWS_PROFILE}"
    if [[ "$_CTX_AWS_EXPIRY" == "expired" ]]; then
      _ftr+="  ${C_RED}⏱ expired${C_RESET}"
    elif [[ -n "$_CTX_AWS_EXPIRY" ]]; then
      _ftr+="  ${C_GREEN}⏱ ${_CTX_AWS_EXPIRY}${C_RESET}"
    fi
  fi
```

Replace with:
```bash
  if [[ -n "$AWS_SESSION_PROFILE" || "$AWS_SESSION_STATUS" != "unknown" ]]; then
    _ftr+="  ${C_DIM}│${C_RESET}  ${C_CYAN}☁${C_RESET} ${AWS_SESSION_PROFILE:-<none>}"
    local _ctx_acct _glyph _detail
    _ctx_acct=$(aws_session_context_account)
    case "$AWS_SESSION_STATUS" in
      ok)
        if [[ -n "$_ctx_acct" && -n "$AWS_SESSION_ACCOUNT" && "$_ctx_acct" != "$AWS_SESSION_ACCOUNT" ]]; then
          _glyph="${C_YELLOW}⚠${C_RESET}"
          _detail="${C_YELLOW}mismatch ⟶ ${_ctx_acct}${C_RESET}"
        else
          _glyph="${C_GREEN}✓${C_RESET}"
          _detail="${C_DIM}${AWS_SESSION_ACCOUNT}${C_RESET}"
        fi
        ;;
      expired)
        _glyph="${C_RED}✗${C_RESET}"
        _detail="${C_RED}expired${C_RESET}"
        ;;
      no-aws)
        _glyph="${C_DIM}–${C_RESET}"
        _detail="${C_DIM}no aws${C_RESET}"
        ;;
      *)
        _glyph="${C_DIM}…${C_RESET}"
        _detail="${C_DIM}validating${C_RESET}"
        ;;
    esac
    _ftr+=" ${_glyph} ${_detail}"
  fi
```

- [ ] **Step 2: Ensure `$C_YELLOW` exists**

Run: `grep -n "C_YELLOW\b" lib/theme.sh`
Expected: at least one match defining `C_YELLOW`. If none, add to `lib/theme.sh`:
```bash
C_YELLOW=$'\033[33m'
```

- [ ] **Step 3: Syntax check**

Run: `bash -n lib/ui.sh && bash -n lib/theme.sh`
Expected: no output.

- [ ] **Step 4: Smoke run**

Run: `./kubekit.sh`
Expected: footer shows `☁ <profile> … validating` initially (status `unknown` at startup). Navigate any menu, then return; status stays `… validating` until Task 6 wires the validation trigger. Press Q to exit cleanly.

- [ ] **Step 5: Commit**

```bash
git add lib/ui.sh lib/theme.sh
git commit -m "feat(ui): footer reads AWS_SESSION_* with status glyphs + mismatch detection"
```

---

## Task 6: Periodic validation via `_tick`

**Files:**
- Modify: `lib/animation.sh` (function `_tick`, ~line 187-197)

- [ ] **Step 1: Add a validation hook to `_tick`**

Find:
```bash
_TICK_COUNT=0
_tick() {
  _update_anim || true
  _update_spinner || true
  _shimmer_tick || true
  _TICK_COUNT=$((_TICK_COUNT + 1))
  if ((_TICK_COUNT % 150 == 0)); then
    _update_ttl || true
    _redraw_footer || true
  fi
  return 0
```

Replace with:
```bash
_TICK_COUNT=0
_tick() {
  _update_anim || true
  _update_spinner || true
  _shimmer_tick || true
  _TICK_COUNT=$((_TICK_COUNT + 1))
  if ((_TICK_COUNT % 150 == 0)); then
    _update_ttl || true
    # Refresh AWS session every ~150 ticks (~30s of activity). Cheap thanks to
    # the 60s TTL guard inside aws_session_validate.
    if declare -F aws_session_validate &>/dev/null; then
      aws_session_validate || true
    fi
    _redraw_footer || true
  fi
  return 0
```

- [ ] **Step 2: Trigger one validate at startup, non-blocking**

In `kubekit.sh`, find:
```bash
  _enter_alt_screen
  draw_chrome
```
Insert immediately before `_enter_alt_screen`:
```bash
  # Kick off the first AWS validate so the footer becomes accurate within seconds.
  aws_session_validate || true
```

- [ ] **Step 3: Syntax check**

Run: `bash -n lib/animation.sh kubekit.sh`
Expected: no output.

- [ ] **Step 4: Manual run**

Run: `./kubekit.sh`
Expected scenarios depending on shell state:
- With valid SSO + matching context: footer shows `☁ <profile> ✓ <12-digit-account>` within 1 second.
- With kubectl ctx `production-eks` but profile `stage` (the bug case): footer shows `☁ stage ⚠ mismatch ⟶ 963758802620`.
- With expired SSO: footer shows `☁ <profile> ✗ expired`.

Exit cleanly with Q.

- [ ] **Step 5: Commit**

```bash
git add lib/animation.sh kubekit.sh
git commit -m "feat(aws-session): validate on startup and every ~30s via tick"
```

---

## Task 7: DB cache redesign — in-memory only, keyed on validated identity

**Files:**
- Modify: `lib/db_forward.sh` (replace `_DB_DISCOVERED` block + `_discover_db_targets`)

- [ ] **Step 1: Replace cache state**

Find this near the top of `lib/db_forward.sh`:
```bash
_DB_DISCOVERED=()
```
Replace with:
```bash
# In-memory DB discovery cache. Keyed by the validated identity at fetch time.
# Any auth change zeros these via aws_session_context_changed.
DB_CACHE_ACCOUNT=""
DB_CACHE_REGION=""
DB_CACHE_ENTRIES=()
DB_CACHE_TS=0
DB_CACHE_ERROR=""

_DB_CACHE_TTL=60
```

- [ ] **Step 2: Rewrite `_discover_db_targets`**

Find the entire `_discover_db_targets()` function and replace it with:

```bash
# Populate DB_CACHE_* from AWS RDS for the current session.
# Honors a 60s in-memory TTL keyed on (account, region). Failures set
# DB_CACHE_ERROR and do NOT bump DB_CACHE_TS, so the next call retries.
_discover_db_targets() {
  if [[ "$AWS_SESSION_STATUS" != "ok" ]]; then
    return 0
  fi

  local now
  now=$(date +%s)

  if [[ "$DB_CACHE_ACCOUNT" == "$AWS_SESSION_ACCOUNT" \
     && "$DB_CACHE_REGION"  == "$AWS_SESSION_REGION" \
     && $((now - DB_CACHE_TS)) -lt $_DB_CACHE_TTL ]]; then
    return 0
  fi

  local args=()
  [[ -n "$AWS_SESSION_PROFILE" ]] && args+=(--profile "$AWS_SESSION_PROFILE")
  args+=(--region "$AWS_SESSION_REGION" --output text)
  args+=(--cli-connect-timeout 3 --cli-read-timeout 8)

  local clusters instances err_file rc=0
  err_file=$(mktemp)

  clusters=$(aws rds describe-db-clusters "${args[@]}" \
    --query 'DBClusters[].[DBClusterIdentifier,Endpoint,Port]' 2>"$err_file") || rc=$?
  if (( rc != 0 )); then
    DB_CACHE_ERROR=$(head -n1 "$err_file" 2>/dev/null)
    rm -f "$err_file"
    return 1
  fi

  instances=$(aws rds describe-db-instances "${args[@]}" \
    --query 'DBInstances[?DBClusterIdentifier==`null`].[DBInstanceIdentifier,Endpoint.Address,Endpoint.Port]' 2>"$err_file") || rc=$?
  if (( rc != 0 )); then
    DB_CACHE_ERROR=$(head -n1 "$err_file" 2>/dev/null)
    rm -f "$err_file"
    return 1
  fi
  rm -f "$err_file"

  local id endpoint port entry
  local tag="${AWS_SESSION_PROFILE:-default}"
  DB_CACHE_ENTRIES=()

  if [[ -n "$clusters" ]]; then
    while IFS=$'\t' read -r id endpoint port; do
      [[ -z "$id" || -z "$endpoint" || "$endpoint" == "None" ]] && continue
      entry="${id} (${AWS_SESSION_REGION}) [${tag}]|${endpoint}|${port:-5432}"
      DB_CACHE_ENTRIES+=("$entry")
    done <<< "$clusters"
  fi

  if [[ -n "$instances" ]]; then
    while IFS=$'\t' read -r id endpoint port; do
      [[ -z "$id" || -z "$endpoint" || "$endpoint" == "None" ]] && continue
      entry="${id} (${AWS_SESSION_REGION}) [${tag}]|${endpoint}|${port:-5432}"
      DB_CACHE_ENTRIES+=("$entry")
    done <<< "$instances"
  fi

  DB_CACHE_ACCOUNT="$AWS_SESSION_ACCOUNT"
  DB_CACHE_REGION="$AWS_SESSION_REGION"
  DB_CACHE_TS="$now"
  DB_CACHE_ERROR=""
  return 0
}
```

- [ ] **Step 3: Drop the old resolve helpers**

In `lib/db_forward.sh`, delete the functions `_db_resolve_profile` and `_db_resolve_region` entirely — `aws_session_resolve` owns this now. Verify they're no longer referenced:

Run: `grep -n "_db_resolve_profile\|_db_resolve_region" lib/db_forward.sh`
Expected: no output.

- [ ] **Step 4: Syntax check**

Run: `bash -n lib/db_forward.sh`
Expected: no output.

- [ ] **Step 5: Commit**

```bash
git add lib/db_forward.sh
git commit -m "refactor(db): in-memory cache keyed on validated identity, no disk writes"
```

---

## Task 8: DB picker uses `aws_session_ensure` and surfaces errors

**Files:**
- Modify: `lib/db_forward.sh` (function `db_forward`, ~line 100-130)

- [ ] **Step 1: Replace the picker entry block**

Find this block in `db_forward()` (the top of the function through the "Custom endpoint" line):
```bash
db_forward() {
  header "Database Tunnel"

  # Build target list: config entries + auto-discovered + custom.
  local _db_options=()
  if [[ ${#CFG_DB_TARGETS[@]} -gt 0 ]]; then
    for _dbt in "${CFG_DB_TARGETS[@]}"; do
      _db_options+=("$_dbt")
    done
  fi

  dim "Discovering databases via AWS RDS..."
  _discover_db_targets
  clear_content
  header "Database Tunnel"

  if [[ ${#_DB_DISCOVERED[@]} -gt 0 ]]; then
    local _new_host _ex _dup
    for _dbt in "${_DB_DISCOVERED[@]}"; do
      _new_host="${_dbt#*|}"
      _new_host="${_new_host%%|*}"
      _dup=false
      if [[ ${#_db_options[@]} -gt 0 ]]; then
        for _ex in "${_db_options[@]}"; do
          [[ "$_ex" == *"|${_new_host}|"* ]] && _dup=true && break
        done
      fi
      $_dup || _db_options+=("$_dbt")
    done
  fi

  _db_options+=("Custom endpoint")
```

Replace with:
```bash
db_forward() {
  header "Database Tunnel"

  # Start from configured targets.
  local _db_options=()
  if [[ ${#CFG_DB_TARGETS[@]} -gt 0 ]]; then
    for _dbt in "${CFG_DB_TARGETS[@]}"; do
      _db_options+=("$_dbt")
    done
  fi

  # Gate: try to ensure an authenticated AWS session before discovery.
  local _can_discover=0
  if aws_session_ensure; then
    _can_discover=1
  fi

  if (( _can_discover )); then
    dim "Discovering databases in account ${AWS_SESSION_ACCOUNT} / region ${AWS_SESSION_REGION}..."
    _discover_db_targets || true
    clear_content
    header "Database Tunnel"
  fi

  # Identity line: show what discovery used (or why it didn't run).
  case "$AWS_SESSION_STATUS" in
    ok)
      local _ctx_acct
      _ctx_acct=$(aws_session_context_account)
      if [[ -n "$_ctx_acct" && "$_ctx_acct" != "$AWS_SESSION_ACCOUNT" ]]; then
        warn "Profile '${AWS_SESSION_PROFILE}' is in account ${AWS_SESSION_ACCOUNT}, but kubectl context targets ${_ctx_acct}."
      else
        dim "Profile: ${AWS_SESSION_PROFILE}  •  Account: ${AWS_SESSION_ACCOUNT}  •  Region: ${AWS_SESSION_REGION}"
      fi
      ;;
    expired) err "AWS session expired. Auto-discovery skipped. Pick a configured target or Custom endpoint." ;;
    no-aws)  dim  "No AWS session available. Showing configured targets only." ;;
    *)       dim  "AWS session unknown. Showing configured targets only." ;;
  esac

  # Merge discovered entries with dedup-by-hostname.
  if [[ ${#DB_CACHE_ENTRIES[@]} -gt 0 ]]; then
    local _new_host _ex _dup
    for _dbt in "${DB_CACHE_ENTRIES[@]}"; do
      _new_host="${_dbt#*|}"
      _new_host="${_new_host%%|*}"
      _dup=false
      if [[ ${#_db_options[@]} -gt 0 ]]; then
        for _ex in "${_db_options[@]}"; do
          [[ "$_ex" == *"|${_new_host}|"* ]] && _dup=true && break
        done
      fi
      $_dup || _db_options+=("$_dbt")
    done
  fi

  if [[ -n "$DB_CACHE_ERROR" ]]; then
    err "Discovery error: ${DB_CACHE_ERROR}"
  fi

  _db_options+=("Custom endpoint")
```

- [ ] **Step 2: Syntax check**

Run: `bash -n lib/db_forward.sh`
Expected: no output.

- [ ] **Step 3: Manual verification matrix**

Run `./kubekit.sh` and enter Database menu under each shell state:

| Shell state | Expected behavior |
|---|---|
| Valid SSO, matching context+profile account | `Profile: … • Account: … • Region: …` dim line, picker lists configured + discovered + Custom |
| Valid SSO, mismatch (ctx=prod-eks, profile=stage) | Yellow warning line about the mismatch, picker still works, discovered entries tagged `[stage]` |
| Expired SSO | `gum confirm` asks to run `aws sso login`; on Yes, login flow runs, then picker shows discovered prod entries; on No, picker shows configured + Custom only with red expired line |
| `unset AWS_PROFILE` and no creds | Dim "No AWS session available" line, picker shows configured + Custom |
| Re-enter the picker within 60s | No `Discovering…` line (cache hit) |
| Switch kubectl context (separate shell, then re-enter menu) | Cache invalidated, fresh discovery |

- [ ] **Step 4: Commit**

```bash
git add lib/db_forward.sh
git commit -m "feat(db): gate discovery via aws_session_ensure, surface errors + mismatch"
```

---

## Task 9: `session.sh` reuses `aws_session_login` for kubectl-cluster recovery

**Files:**
- Modify: `lib/session.sh` (function `refresh_aws_sso`, ~line 35-55)

- [ ] **Step 1: Read the current function**

Run: `sed -n '1,63p' lib/session.sh`
Identify the SSO-refresh branches that call `aws sso login --profile`.

- [ ] **Step 2: Replace direct sso-login calls with `aws_session_login`**

Find the inner block in `refresh_aws_sso`:
```bash
    if [[ -n "$profile" ]]; then
      warn "Can't reach cluster — refreshing SSO session ($profile)..."
      local rc=0
      run_interactive aws sso login --profile "$profile" || rc=$?
    else
      warn "Can't reach cluster. No AWS profile found."
      local profile_sel
      profile_sel=$(pick_aws_profile) || return 1
      warn "Logging in as $profile_sel..."
      local rc=0
      run_interactive aws sso login --profile "$profile_sel" || rc=$?
    fi
```

Replace with:
```bash
    if [[ -n "$profile" ]]; then
      warn "Can't reach cluster — refreshing SSO session ($profile)..."
      AWS_SESSION_PROFILE="$profile"
      aws_session_login || true
    else
      warn "Can't reach cluster. No AWS profile found."
      local profile_sel
      profile_sel=$(pick_aws_profile) || return 1
      warn "Logging in as $profile_sel..."
      AWS_SESSION_PROFILE="$profile_sel"
      aws_session_login || true
    fi
```

- [ ] **Step 3: Syntax check**

Run: `bash -n lib/session.sh`
Expected: no output.

- [ ] **Step 4: Manual verification**

Force the cluster-unreachable path by setting `KUBECONFIG=/dev/null` then running `./kubekit.sh` and entering any menu that touches kubectl. Expected: warn line shows, SSO login flow runs via `aws_session_login`, and `AWS_SESSION_STATUS` becomes `ok` (visible in footer) after success.

- [ ] **Step 5: Commit**

```bash
git add lib/session.sh
git commit -m "refactor(session): delegate kubectl-recovery sso login to aws_session_login"
```

---

## Task 10: AWS actions gate on `aws_session_ensure`

**Files:**
- Modify: `lib/actions_aws.sh`

- [ ] **Step 1: Inspect current action entry points**

Run: `grep -n "^[a-z_]\+()" lib/actions_aws.sh`
Note each function name (e.g. `action_sso_login`, `action_set_eks_context`, `action_browse_s3`, etc.).

- [ ] **Step 2: For each AWS-using action, gate on `aws_session_ensure`**

For each action function found in Step 1, add this near the top — **except** for `action_sso_login` itself, which performs the login. Pattern:

```bash
action_foo() {
  if ! aws_session_ensure; then
    case "$AWS_SESSION_STATUS" in
      no-aws) err "No AWS session available. Configure a profile and try again." ;;
      *)      err "AWS session not ready: ${AWS_SESSION_ERROR:-unknown}" ;;
    esac
    pause
    return
  fi
  # …existing body…
}
```

Skip actions that only run interactive `aws sso login` or `aws configure sso` (those are recovery flows themselves).

- [ ] **Step 3: After context change actions (e.g. EKS context set), call `aws_session_context_changed`**

In `action_set_eks_context` (or whichever function calls `aws eks update-kubeconfig`), append after the successful update:

```bash
  aws_session_context_changed
  _redraw_footer
```

- [ ] **Step 4: Syntax check**

Run: `bash -n lib/actions_aws.sh`
Expected: no output.

- [ ] **Step 5: Manual verification**

Run `./kubekit.sh`. Navigate to AWS menu with an expired session. Each action should prompt to re-login via `gum confirm` instead of failing silently. After EKS context update, the footer's profile/account display should refresh.

- [ ] **Step 6: Commit**

```bash
git add lib/actions_aws.sh
git commit -m "feat(actions): gate AWS actions on aws_session_ensure, refresh footer on context change"
```

---

## Task 11: Cluster context switch triggers `aws_session_context_changed`

**Files:**
- Modify: `lib/actions_cluster.sh`

- [ ] **Step 1: Find the context-switch action**

Run: `grep -n "use-context\|set-context" lib/actions_cluster.sh`

- [ ] **Step 2: Add the hook after a successful switch**

In the function that calls `kubectl config use-context "$ctx"`, after the successful call append:

```bash
  if declare -F aws_session_context_changed &>/dev/null; then
    aws_session_context_changed
  fi
  _refresh_ctx || true
  _redraw_footer || true
```

- [ ] **Step 3: Syntax check**

Run: `bash -n lib/actions_cluster.sh`
Expected: no output.

- [ ] **Step 4: Manual verification**

Run `./kubekit.sh`. Switch from a non-EKS context to `production-eks`. Footer should immediately show `… validating`, then resolve to `✓` / `⚠` / `✗` within ~1s.

- [ ] **Step 5: Commit**

```bash
git add lib/actions_cluster.sh
git commit -m "feat(cluster): invalidate AWS session state on kubectl context switch"
```

---

## Task 12: End-to-end verification + final commit

- [ ] **Step 1: Run the full manual scenario matrix from the spec**

For each of the seven scenarios in spec section "Testing", run `./kubekit.sh` and confirm behavior matches:

1. Happy path — footer `✓ <account>`, prod entries listed.
2. Mismatch — footer `⚠ mismatch ⟶ <account>`, entries tagged `[stage]`.
3. Expired SSO — `gum confirm` prompts; on accept, login + re-discover.
4. No aws cli (test by temporarily `alias aws=/bin/false` in a subshell) — discovery skipped, configured + Custom still work.
5. Profile switch mid-session — footer updates, cache cleared.
6. Cache TTL — re-enter picker within 60s, no `Discovering…` line.
7. Legacy cache cleanup — `ls ~/.local/state/kubekit/db_cache_* 2>/dev/null` returns no results after first launch.

- [ ] **Step 2: Audit for project-specific leakage**

Run:
```bash
grep -rnE "soccer|oddsforge|sportcontext|draftkings|production-eks|staging-eks|963758802620|072092343946|cteiwyccsak7|chm826uieyqw" \
  --include="*.sh" --include="*.md" --include="*.example" . 2>/dev/null | grep -v "^./.git/"
```
Expected: no output. The implementation must stay generic.

- [ ] **Step 3: Update `CHANGELOG.md`**

Append under a new "Unreleased" section:
```markdown
## [Unreleased]

### Added
- Live AWS session state with `aws sts get-caller-identity` validation, surfaced in the footer (`✓` ok / `⚠` mismatch / `✗` expired / `–` no-aws).
- Automatic `aws sso login` prompt when an expired session is detected on entering any AWS-using menu.
- Account-mismatch detection between kubectl context ARN and resolved AWS profile.
- DB discovery cache invalidates automatically when AWS session identity changes.

### Changed
- DB discovery cache moved from disk to in-memory; legacy `~/.local/state/kubekit/db_cache_*` files are removed on first launch.
- Footer profile segment now requires successful validation before showing as green.

### Removed
- Hardcoded staging Aurora fallback in the DB tunnel picker.
- On-disk DB cache directory writes.
```

- [ ] **Step 4: Final commit**

```bash
git add CHANGELOG.md
git commit -m "docs(changelog): record P1 — AWS session state + discovery"
```

- [ ] **Step 5: Verify clean tree**

Run: `git status`
Expected: `nothing to commit, working tree clean`.

Run: `git log --oneline -15`
Expected: 12 new commits (one per task), most recent on top.

---

## Self-review

- **Spec coverage:**
  - Component 1 (`aws_session.sh`): Tasks 1-3 ✓
  - Component 2 (footer + status): Tasks 5-6 ✓
  - Component 3 (DB cache redesign): Task 7 ✓
  - Component 4 (DB picker behavior): Task 8 ✓
  - Component 5 (wire-up): Tasks 4, 9, 10, 11 ✓
  - Error handling matrix: covered in Task 8 verification + Task 12 scenarios ✓
  - Legacy cleanup: Task 3 step 1 (in module load) ✓
  - Testing scenarios from spec: Task 12 ✓
- **Placeholder scan:** No TBD / TODO / "handle edge cases". All code blocks complete.
- **Type/name consistency:**
  - `AWS_SESSION_*` names match across tasks ✓
  - `DB_CACHE_*` names match across Tasks 3, 7, 8 ✓
  - `aws_session_context_changed` called from Tasks 10, 11 with matching signature ✓
  - `aws_session_context_account` defined Task 1, used Tasks 5, 8 ✓

Plan looks complete.
