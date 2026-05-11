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
