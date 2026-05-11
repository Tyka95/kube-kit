# ── AWS actions ───────────────────────────────────────────────────────────────

sso_login() {
  push_breadcrumb "SSO Login"
  set_keyhints "? help"
  draw_chrome
  header "SSO Login"
  local profile
  if ! profile=$(pick_aws_profile); then
    pop_breadcrumb
    clear_keyhints
    return
  fi
  show_cmd "aws sso login --profile $profile"
  # Exit alt screen for interactive browser auth flow
  printf '\033[?25h' >&3
  _exit_alt_screen
  local rc=0
  aws sso login --profile "$profile" || rc=$?
  echo "" >/dev/tty
  read -rsn1 -p "Press any key to continue..." </dev/tty >/dev/tty 2>/dev/null || true
  _enter_alt_screen
  draw_chrome
  if ((rc == 0)); then
    ok "Logged in as $profile."
    AWS_SESSION_PROFILE="$profile"
    aws_session_context_changed
    aws_session_validate 1 || true
  else
    err "SSO login failed."
  fi
  _refresh_ctx || true
  _redraw_footer || true
  divider
  pop_breadcrumb
  clear_keyhints
}

switch_context() {
  push_breadcrumb "Context"
  set_keyhints "? help"
  draw_chrome
  header "Switch Context"
  local contexts
  contexts=$(kubectl config get-contexts -o name 2>/dev/null | sort) || true
  if [[ -z "$contexts" ]]; then
    err "No contexts configured."
    pop_breadcrumb
    clear_keyhints
    return
  fi

  local current
  current=$(kubectl config current-context 2>/dev/null) || true

  # Build picker items. For EKS ARN contexts (arn:aws:eks:<region>:<acct>:cluster/<name>),
  # show the short cluster name as the label, account+region in the description,
  # and "current" in the meta column. Non-EKS contexts (kind, docker-desktop,
  # etc.) show the raw name in the label.
  local _ctx_items=() _ctx_line _suffix _label _desc
  # Parallel array: maps shown-label → full context name to switch to.
  declare -a _ctx_full=()
  while IFS= read -r _ctx_line; do
    [[ -z "$_ctx_line" ]] && continue
    _suffix=""
    [[ "$_ctx_line" == "$current" ]] && _suffix="current"
    if [[ "$_ctx_line" =~ ^arn:aws:eks:([a-z0-9-]+):([0-9]{12}):cluster/(.+)$ ]]; then
      _label="${BASH_REMATCH[3]}"
      _desc="${BASH_REMATCH[1]}  ${BASH_REMATCH[2]}"
    else
      _label="$_ctx_line"
      _desc="kubectl"
    fi
    _ctx_items+=("${_label}|${_desc}|${_suffix}")
    _ctx_full+=("$_ctx_line")
  done <<< "$contexts"

  PICKER_BINDS=()
  if ! choose_menu "Switch kubectl context" "${_ctx_items[@]}"; then
    pop_breadcrumb
    clear_keyhints
    return
  fi
  if [[ "$PICKER_RESULT_KIND" != "select" ]]; then
    pop_breadcrumb
    clear_keyhints
    return
  fi
  # Map the chosen label back to the full kubectl context name.
  local _picked_label="$PICKER_RESULT_VALUE"
  local selected=""
  local _i
  for _i in "${!_ctx_items[@]}"; do
    if [[ "${_ctx_items[$_i]%%|*}" == "$_picked_label" ]]; then
      selected="${_ctx_full[$_i]}"
      break
    fi
  done
  [[ -z "$selected" ]] && selected="$_picked_label"

  kubectl config use-context "$selected" >/dev/null 2>&1
  ok "Switched to $selected."
  aws_session_context_changed
  _refresh_ctx
  aws_session_validate 1 || true
  _redraw_footer || true
  divider
  pop_breadcrumb
  clear_keyhints
}

connect_cluster() {
  push_breadcrumb "EKS Connect"
  set_keyhints "? help"
  draw_chrome
  header "Connect EKS Cluster"
  local profile
  if ! profile=$(pick_aws_profile); then
    pop_breadcrumb
    clear_keyhints
    return
  fi
  AWS_SESSION_PROFILE="$profile"
  if ! aws_session_ensure; then
    err "AWS session not ready for profile '$profile': ${AWS_SESSION_ERROR:-unavailable}."
    pause
    pop_breadcrumb
    clear_keyhints
    return
  fi

  local regions=()
  if [[ ${#CFG_AWS_REGIONS[@]} -gt 0 ]]; then
    regions=("${CFG_AWS_REGIONS[@]}")
  else
    regions=("eu-west-1" "eu-west-2" "us-east-1" "us-west-2")
  fi
  local all_clusters=""

  dim "Scanning ${#regions[@]} regions for EKS clusters..."
  echo "" >&3

  # Scan all regions in parallel
  local tmpdir
  tmpdir=$(mktemp -d)
  for region in "${regions[@]}"; do
    (
      local result
      result=$(aws eks list-clusters --region "$region" --profile "$profile" \
        --query 'clusters[]' --output text 2>/dev/null) || true
      if [[ -n "$result" ]]; then
        echo "$result" | tr '\t' '\n' | while read -r c; do
          [[ -n "$c" ]] && echo "${c} (${region})"
        done
      fi
    ) > "${tmpdir}/${region}" &
  done
  wait

  for region in "${regions[@]}"; do
    [[ -f "${tmpdir}/${region}" ]] && all_clusters+=$(cat "${tmpdir}/${region}")$'\n'
  done
  rm -rf "$tmpdir"

  all_clusters=$(echo "$all_clusters" | sed '/^$/d' | sort)
  if [[ -z "$all_clusters" ]]; then
    err "No clusters found in any region."
    pop_breadcrumb
    clear_keyhints
    return
  fi

  local _eks_items=() _eks_line _name _region
  while IFS= read -r _eks_line; do
    [[ -z "$_eks_line" ]] && continue
    _name="${_eks_line%% (*}"
    _region="${_eks_line##*(}"
    _region="${_region%)}"
    _eks_items+=("${_name}|eks|${_region}")
  done <<< "$all_clusters"

  PICKER_BINDS=()
  if ! choose_menu "EKS Cluster" "${_eks_items[@]}"; then
    pop_breadcrumb
    clear_keyhints
    return
  fi
  if [[ "$PICKER_RESULT_KIND" != "select" ]]; then
    pop_breadcrumb
    clear_keyhints
    return
  fi
  local selected_name="$PICKER_RESULT_VALUE"

  local cluster="$selected_name" region=""
  # Recover region by matching the original "<name> (<region>)" line.
  region=$(printf '%s\n' "$all_clusters" | grep -F "${selected_name} (" | head -n1)
  region="${region##*(}"
  region="${region%)}"

  show_cmd "aws eks update-kubeconfig --region $region --name $cluster --profile $profile"
  aws eks update-kubeconfig --region "$region" --name "$cluster" --profile "$profile" 2>&1 | while IFS= read -r line; do
    dim "$line"
  done
  ok "Context set to $cluster ($region)."
  aws_session_context_changed
  _refresh_ctx
  aws_session_validate 1 || true
  _redraw_footer || true
  divider
  pop_breadcrumb
  clear_keyhints
}

list_buckets() {
  push_breadcrumb "S3 Buckets"
  set_keyhints "? help"
  draw_chrome
  header "S3 Buckets"
  local profile
  if ! profile=$(pick_aws_profile); then
    pop_breadcrumb
    clear_keyhints
    return
  fi
  AWS_SESSION_PROFILE="$profile"
  if ! aws_session_ensure; then
    err "AWS session not ready for profile '$profile': ${AWS_SESSION_ERROR:-unavailable}."
    pause
    pop_breadcrumb
    clear_keyhints
    return
  fi
  show_cmd "aws s3 ls --profile $profile"
  echo "" >&3
  aws s3 ls --profile "$profile" >&3
  echo "" >&3
  divider
  pop_breadcrumb
  clear_keyhints
}
