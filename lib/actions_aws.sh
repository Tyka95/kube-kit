# ── AWS actions ───────────────────────────────────────────────────────────────

sso_login() {
  header "SSO Login"
  local profile
  profile=$(pick_aws_profile) || return
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
}

switch_context() {
  header "Switch Context"
  local contexts
  contexts=$(kubectl config get-contexts -o name 2>/dev/null | sort) || true
  [[ -z "$contexts" ]] && { err "No contexts configured."; return; }

  local current
  current=$(kubectl config current-context 2>/dev/null) || true

  local selected
  selected=$(echo "$contexts" | gum filter --placeholder "Type to search..." --header "  Switch kubectl context (current: ${current:-none})") || return

  kubectl config use-context "$selected" >/dev/null 2>&1
  ok "Switched to $selected."
  aws_session_context_changed
  _refresh_ctx
  aws_session_validate 1 || true
  _redraw_footer || true
  divider
}

connect_cluster() {
  header "Connect EKS Cluster"
  local profile
  profile=$(pick_aws_profile) || return
  AWS_SESSION_PROFILE="$profile"
  if ! aws_session_ensure; then
    err "AWS session not ready for profile '$profile': ${AWS_SESSION_ERROR:-unavailable}."
    pause
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
  [[ -z "$all_clusters" ]] && { err "No clusters found in any region."; return; }

  local selected
  selected=$(echo "$all_clusters" | gum filter --placeholder "Type to search..." --header "  EKS Cluster") || return

  local cluster region
  cluster="${selected%% (*}"
  region="${selected##*(}"
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
}

list_buckets() {
  header "S3 Buckets"
  local profile
  profile=$(pick_aws_profile) || return
  AWS_SESSION_PROFILE="$profile"
  if ! aws_session_ensure; then
    err "AWS session not ready for profile '$profile': ${AWS_SESSION_ERROR:-unavailable}."
    pause
    return
  fi
  show_cmd "aws s3 ls --profile $profile"
  echo "" >&3
  aws s3 ls --profile "$profile" >&3
  echo "" >&3
  divider
}
