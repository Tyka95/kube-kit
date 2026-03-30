# ── Session helpers ───────────────────────────────────────────────────────────

ensure_aws_session() {
  local profile="$1"
  if ! aws sts get-caller-identity --profile "$profile" &>/dev/null; then
    warn "Session expired for $profile — logging in..."
    printf '\033[?25h' >&3
    _exit_alt_screen
    local rc=0
    aws sso login --profile "$profile" || rc=$?
    echo "" >/dev/tty
    read -rsn1 -p "Press any key to continue..." </dev/tty >/dev/tty 2>/dev/null || true
    _enter_alt_screen
    draw_chrome
    if ! aws sts get-caller-identity --profile "$profile" &>/dev/null; then
      err "Login failed for $profile."
      return 1
    fi
    ok "Session refreshed."
  fi
}

ensure_kube_context() {
  # Fast check: can we reach the API server?
  kubectl get --raw /readyz --request-timeout=5s &>/dev/null && return 0

  local ctx
  ctx=$(kubectl config current-context 2>/dev/null) || true

  if [[ -z "$ctx" ]]; then
    warn "No cluster context configured."
    if gum confirm "Connect via EKS?"; then
      connect_cluster
    else
      return 1
    fi
  else
    # Extract AWS_PROFILE from kubeconfig exec env
    local profile=""
    profile=$(kubectl config view --minify -o jsonpath='{.users[0].user.exec.env[?(@.name=="AWS_PROFILE")].value}' 2>/dev/null) || true
    [[ -z "$profile" ]] && profile="${AWS_PROFILE:-}"

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
  fi

  # Re-check after login
  if ! kubectl get --raw /readyz --request-timeout=5s &>/dev/null; then
    err "Still can't reach cluster."
    return 1
  fi
  ok "Connected."
}
