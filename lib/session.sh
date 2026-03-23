# ── Session helpers ───────────────────────────────────────────────────────────

ensure_aws_session() {
  local profile="$1"
  if ! aws sts get-caller-identity --profile "$profile" &>/dev/null; then
    warn "Session expired for $profile — logging in..." >&2
    aws sso login --profile "$profile" >&2
    if ! aws sts get-caller-identity --profile "$profile" &>/dev/null; then
      err "Login failed for $profile." >&2
      return 1
    fi
    ok "Session refreshed." >&2
  fi
}

ensure_kube_context() {
  kubectl cluster-info &>/dev/null && return 0
  local ctx
  ctx=$(kubectl config current-context 2>/dev/null) || true

  if [[ -z "$ctx" ]]; then
    warn "No cluster context configured." >&2
    if gum confirm "Connect via EKS?" >&2; then
      connect_cluster >&2
    else
      return 1
    fi
  else
    warn "Can't reach cluster ($ctx). Session may be expired." >&2
    local profile
    profile=$(pick_aws_profile) || return 1
    warn "Logging in as $profile..." >&2
    aws sso login --profile "$profile" >&2 || { err "SSO login failed." >&2; return 1; }
    ok "SSO login complete." >&2
  fi

  kubectl cluster-info &>/dev/null || { err "Still can't reach cluster." >&2; return 1; }
  ok "Connected." >&2
}
