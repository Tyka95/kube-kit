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
AWS_SESSION_CTX_ACCOUNT=""    # cached: account id from current kubectl context ARN

# Resolve PROFILE and REGION from kubeconfig exec env > env vars > kubekit config.
# No hardcoded defaults — empty result means "unknown".
aws_session_resolve() {
  local ctx region="" ctx_acct=""
  ctx=$(kubectl config current-context 2>/dev/null) || true

  local profile=""
  profile=$(kubectl config view --minify -o jsonpath='{.users[0].user.exec.env[?(@.name=="AWS_PROFILE")].value}' 2>/dev/null) || true
  [[ -z "$profile" ]] && profile="${AWS_PROFILE:-${AWS_DEFAULT_PROFILE:-${CFG_DEFAULT_PROFILE:-}}}"
  AWS_SESSION_PROFILE="$profile"

  if [[ "$ctx" =~ arn:aws:eks:([a-z0-9-]+):([0-9]{12}): ]]; then
    region="${BASH_REMATCH[1]}"
    ctx_acct="${BASH_REMATCH[2]}"
  fi
  [[ -z "$region" ]] && region="${AWS_REGION:-${AWS_DEFAULT_REGION:-}}"
  if [[ -z "$region" && ${#CFG_AWS_REGIONS[@]} -gt 0 ]]; then
    region="${CFG_AWS_REGIONS[0]}"
  fi
  AWS_SESSION_REGION="$region"
  AWS_SESSION_CTX_ACCOUNT="$ctx_acct"
}

# Cached accessor for the EKS context's account id. Refreshed by
# aws_session_resolve / aws_session_context_changed — callers (like
# _footer_bar) should not call kubectl themselves on every redraw.
aws_session_context_account() {
  printf '%s' "$AWS_SESSION_CTX_ACCOUNT"
}

# Validate the current session via sts get-caller-identity.
# Operates on whatever AWS_SESSION_PROFILE/REGION are currently set — does NOT
# re-resolve from environment. Use aws_session_context_changed (or set the
# profile explicitly) before calling if you need to switch identity.
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
  args+=(--cli-connect-timeout 3 --cli-read-timeout 5 --output text --query '[Account,Arn]')

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
        "$err_line" == *"Token has expired"* || "$err_line" == *"SSO session"* || \
        "$err_line" == *"sso-oidc"* || "$err_line" == *"refresh"* ]]; then
    AWS_SESSION_STATUS="expired"
  elif [[ -z "$AWS_SESSION_PROFILE" && -z "${AWS_ACCESS_KEY_ID:-}" ]]; then
    AWS_SESSION_STATUS="no-aws"
  else
    # Unknown failure (config error, network, JMESPath bug, etc.). Don't mask
    # as 'expired' — that hides real problems. Surface as unknown so the footer
    # shows '… validating' and the user sees AWS_SESSION_ERROR.
    AWS_SESSION_STATUS="unknown"
  fi
  return 1
}

# Interactively run `aws sso login --profile <P>` and re-validate.
# Drops out of the alt-screen so the device-code URL is visible, then
# restores chrome after the user returns. Returns 0 if STATUS=ok after login.
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

  printf '\033[?25h' >&3
  if declare -F _exit_alt_screen &>/dev/null; then _exit_alt_screen; fi
  aws sso login --profile "$AWS_SESSION_PROFILE" || true
  echo "" >/dev/tty 2>/dev/null || true
  read -rsn1 -p "Press any key to continue..." </dev/tty >/dev/tty 2>/dev/null || true
  if declare -F _enter_alt_screen &>/dev/null; then _enter_alt_screen; fi
  if declare -F draw_chrome &>/dev/null; then draw_chrome; fi

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
# Use find rather than a glob so zsh's "no matches" error doesn't leak when
# the directory exists but contains nothing matching.
_aws_session_cleanup_legacy() {
  local d="${HOME}/.local/state/kubekit"
  [[ -d "$d" ]] || return 0
  find "$d" -maxdepth 1 -type f -name 'db_cache_*' -exec rm -f {} + 2>/dev/null || true
}
_aws_session_cleanup_legacy

# Initial resolve at module load — populates PROFILE/REGION/CTX_ACCOUNT from
# the current environment so the footer and first validate have something to
# go on. Re-run by aws_session_context_changed when identity changes.
aws_session_resolve
