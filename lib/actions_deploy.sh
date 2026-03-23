# ── Deployment actions ────────────────────────────────────────────────────────

_inspect_deploy() {
  local ns="$1" dep="$2"
  header "Inspect: $dep"
  show_cmd "kubectl describe deployment $dep -n $ns"
  echo "" >&3
  kubectl describe deployment "$dep" -n "$ns" | colorize_k8s >&3
  echo "" >&3
  divider
}

_scale_replicas() {
  local ns="$1" dep="$2"
  local current
  current=$(kubectl get deployment "$dep" -n "$ns" -o jsonpath='{.spec.replicas}' 2>/dev/null)

  header "Scale: $dep"
  dim "Current replicas: $current"
  echo "" >&3

  local count
  count=$(gum input --placeholder "Enter replica count" --prompt "  Replicas: " --header "  Scale $dep (current: $current)") || return
  [[ "$count" =~ ^[0-9]+$ ]] || { err "Invalid number."; return; }

  show_cmd "kubectl scale deployment $dep -n $ns --replicas=$count"
  kubectl scale deployment "$dep" -n "$ns" --replicas="$count" >&3
  echo "" >&3
  ok "Scaled $dep to $count replica(s)."
  divider
}

_rolling_restart() {
  local ns="$1" dep="$2"
  header "Restart: $dep"
  show_cmd "kubectl rollout restart deployment $dep -n $ns"
  kubectl rollout restart deployment "$dep" -n "$ns" >&3
  echo "" >&3
  ok "Rolling restart initiated for $dep."
  divider
}
