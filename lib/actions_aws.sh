# ── AWS actions ───────────────────────────────────────────────────────────────

sso_login() {
  header "SSO Login"
  local profile
  profile=$(pick_aws_profile) || return
  show_cmd "aws sso login --profile $profile"
  aws sso login --profile "$profile" >&3
  echo "" >&3
  ok "Logged in as $profile."
  _refresh_ctx
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

  kubectl config use-context "$selected" >&3
  echo "" >&3
  ok "Switched to $selected."
  _refresh_ctx
  divider
}

connect_cluster() {
  header "Connect EKS Cluster"
  local profile
  profile=$(pick_aws_profile) || return
  ensure_aws_session "$profile" || return

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
  aws eks update-kubeconfig --region "$region" --name "$cluster" --profile "$profile" >&3
  echo "" >&3
  ok "Context set to $cluster ($region)."
  _refresh_ctx
  divider
}

list_buckets() {
  header "S3 Buckets"
  local profile
  profile=$(pick_aws_profile) || return
  ensure_aws_session "$profile" || return
  show_cmd "aws s3 ls --profile $profile"
  echo "" >&3
  aws s3 ls --profile "$profile" >&3
  echo "" >&3
  divider
}
