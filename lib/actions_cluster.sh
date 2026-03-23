# ── Cluster actions ───────────────────────────────────────────────────────────

show_context() {
  header "Current Context"
  local ctx
  ctx=$(kubectl config current-context 2>/dev/null) || true
  if [[ -z "$ctx" ]]; then
    err "No current context set."
    return
  fi
  ok "Context: $ctx"
  echo "" >&3
  show_cmd "kubectl cluster-info"
  kubectl cluster-info 2>/dev/null >&3 || warn "Cannot reach cluster."
  echo "" >&3
  divider
}

show_nodes() {
  kube_output "Node Status" "kubectl get nodes -o wide" \
    kubectl get nodes -o wide
}

show_events() {
  local ns
  ns=$(pick_namespace) || return
  header "Events in $ns (last 30)"
  show_cmd "kubectl get events -n $ns --sort-by='.lastTimestamp'"
  echo "" >&3
  kubectl get events -n "$ns" --sort-by='.lastTimestamp' 2>/dev/null | tail -30 | colorize_k8s >&3
  echo "" >&3
  divider
}

resource_usage() {
  local ns
  ns=$(pick_namespace) || return
  header "Resource Usage in $ns"
  show_cmd "kubectl top pods -n $ns"
  echo "" >&3
  kubectl top pods -n "$ns" 2>/dev/null | colorize_k8s >&3 || warn "Metrics server may not be available."
  echo "" >&3
  divider
}
