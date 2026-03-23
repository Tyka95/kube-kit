# ── DRY kubectl wrappers ─────────────────────────────────────────────────────

kube_output() {
  local title="$1" cmd="$2"; shift 2
  header "$title"
  show_cmd "$cmd"
  echo "" >&3
  "$@" 2>&3 | colorize_k8s >&3
  echo "" >&3
  divider
}

list_resource() {
  local title="$1" resource="$2" flags="${3:-}"
  local ns
  ns=$(pick_namespace) || return
  header "$title in $ns"
  show_cmd "kubectl get $resource -n $ns $flags"
  echo "" >&3
  kubectl get "$resource" -n "$ns" $flags 2>/dev/null | colorize_k8s >&3
  echo "" >&3
  divider
}

with_pod() {
  local title="$1"; shift
  local result
  result=$(pick_pod) || return
  local ns="${result%%/*}" pod="${result#*/}"
  "$@" "$ns" "$pod"
}

with_deployment() {
  local title="$1"; shift
  local result
  result=$(pick_deployment) || return
  local ns="${result%%/*}" dep="${result#*/}"
  "$@" "$ns" "$dep"
}

# ── Pod actions ───────────────────────────────────────────────────────────────

_view_logs() {
  local ns="$1" pod="$2"
  local mode
  mode=$(gum choose --header "  Log output" \
    "Tail 50 lines" "Tail 200 lines" "Stream (follow)" "Full log") || return

  local flags
  case "$mode" in
    "Tail 50 lines")   flags="--tail=50" ;;
    "Tail 200 lines")  flags="--tail=200" ;;
    "Stream (follow)") flags="-f" ;;
    "Full log")        flags="" ;;
  esac

  header "Logs: $pod"
  show_cmd "kubectl logs $pod -n $ns $flags"
  echo "" >&3
  kubectl logs "$pod" -n "$ns" $flags >&3
  drain_stdin
  echo "" >&3
  divider
}

_open_shell() {
  local ns="$1" pod="$2"
  local shell
  shell=$(gum choose --header "  Shell" "/bin/sh" "/bin/bash") || return
  header "Shell: $pod"
  show_cmd "kubectl exec -it $pod -n $ns -- $shell"
  echo "" >&3
  kubectl exec -it "$pod" -n "$ns" -- "$shell" >&3 2>&3 <&3
  drain_stdin
}

_inspect_pod() {
  local ns="$1" pod="$2"
  header "Inspect: $pod"
  show_cmd "kubectl describe pod $pod -n $ns"
  echo "" >&3
  kubectl describe pod "$pod" -n "$ns" | colorize_k8s >&3
  echo "" >&3
  divider
}
