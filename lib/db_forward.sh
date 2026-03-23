# ── DB Port Forward ───────────────────────────────────────────────────────────

DB_PF_POD=""
DB_PF_PID=""
DB_PF_NS=""

db_forward() {
  header "Database Tunnel"
  ensure_kube_context || return

  local target
  target=$(gum choose --header "  Database target" \
    "Staging Aurora (eu-west-1)" \
    "Custom endpoint") || { _redraw_footer; return; }
  clear_content
  header "Database Tunnel"

  # Allow Ctrl+C to cancel prompts and return to menu
  local _db_cancelled=false
  trap '_db_cancelled=true' INT
  trap 'trap - INT' RETURN

  local db_host db_port
  if [[ "$target" == "Staging"* ]]; then
    db_host="staging-rds-aurora-cluster.cluster-chm826uieyqw.eu-west-1.rds.amazonaws.com"
    db_port="5432"
    dim "Target: $target"
  else
    drain_stdin
    printf '  %sHost%s %s(Ctrl+C to cancel)%s: ' "$C_CYAN_B" "$C_RESET" "$C_DIM" "$C_RESET" >&3
    read -r db_host < /dev/tty || true
    $_db_cancelled && { echo "" >&3; return; }
    printf '  %sDB Port%s [%s5432%s]: ' "$C_CYAN_B" "$C_RESET" "$C_DIM" "$C_RESET" >&3
    read -r db_port < /dev/tty || true
    $_db_cancelled && { echo "" >&3; return; }
    db_port="${db_port:-5432}"
  fi
  [[ -z "$db_host" ]] && { err "No host specified."; return; }

  local local_port
  drain_stdin
  printf '  %sLocal port%s [%s15432%s] %s(Ctrl+C to cancel)%s: ' "$C_CYAN_B" "$C_RESET" "$C_DIM" "$C_RESET" "$C_DIM" "$C_RESET" >&3
  read -r local_port < /dev/tty || true
  $_db_cancelled && { echo "" >&3; return; }
  local_port="${local_port:-15432}"

  if [[ ! "$local_port" =~ ^[0-9]+$ ]]; then
    err "Invalid port number."
    return
  fi

  local pod_name="db-forward-$$"
  local ns
  ns=$(kubectl config view --minify -o jsonpath='{.contexts[0].context.namespace}' 2>/dev/null) || true
  ns="${ns:-default}"

  echo "" >&3
  dim "Creating socat pod $pod_name in $ns..."
  show_cmd "kubectl run $pod_name -n $ns --image=alpine/socat --restart=Never -- TCP-LISTEN:$db_port,fork TCP:$db_host:$db_port"
  if ! kubectl run "$pod_name" -n "$ns" --image=alpine/socat --restart=Never -- \
    "TCP-LISTEN:${db_port},fork" "TCP:${db_host}:${db_port}" >&3 2>&3; then
    err "Failed to create socat pod."
    return
  fi

  dim "Waiting for pod to be ready..."
  if ! kubectl wait --for=condition=Ready "pod/${pod_name}" -n "$ns" --timeout=60s >&3 2>&3; then
    err "Pod failed to start. Cleaning up..."
    kubectl delete pod "$pod_name" -n "$ns" --ignore-not-found 2>/dev/null
    return
  fi

  show_cmd "kubectl port-forward pod/$pod_name -n $ns $local_port:$db_port"
  kubectl port-forward "pod/${pod_name}" -n "$ns" "${local_port}:${db_port}" >/dev/null 2>&1 &
  DB_PF_PID=$!
  DB_PF_POD="$pod_name"
  DB_PF_NS="$ns"
  sleep 1

  if kill -0 "$DB_PF_PID" 2>/dev/null; then
    echo "" >&3
    ok "Tunnel active: localhost:$local_port -> $db_host:$db_port"
    echo "" >&3
    printf '  %sConnect with:%s\n' "$C_CYAN_B" "$C_RESET" >&3
    dim "psql \"host=localhost port=$local_port dbname=<db> user=<user>\""
    echo "" >&3
    divider
    warn "Press Ctrl+C to stop and clean up"
    _redraw_footer
    # Wait for Ctrl+C — run cleanup inline in trap to guarantee it runs
    trap '_db_cleanup; trap - INT' INT
    while kill -0 "$DB_PF_PID" 2>/dev/null; do
      perl -e 'select(undef,undef,undef,0.2)' 2>/dev/null || sleep 1 || true
      _tick || true
    done
    trap - INT
    _db_cleanup
  else
    err "Port-forward failed (port $local_port may be in use)."
    _db_cleanup
  fi
}

_db_cleanup() {
  local had_work=0
  if [[ -n "$DB_PF_PID" ]]; then
    had_work=1
    kill "$DB_PF_PID" 2>/dev/null || true
    wait "$DB_PF_PID" 2>/dev/null || true
    DB_PF_PID=""
  fi
  if [[ -n "$DB_PF_POD" ]]; then
    had_work=1
    # Position cursor in content area before printing cleanup messages
    printf '\033[4;1H\033[J' >&3 2>/dev/null || true
    dim "Cleaning up pod $DB_PF_POD..." || true
    kubectl delete pod "$DB_PF_POD" -n "${DB_PF_NS:-default}" --ignore-not-found 2>/dev/null || true
    ok "Deleted pod $DB_PF_POD." || true
    DB_PF_POD=""
  fi
  ((had_work)) && { ok "DB tunnel closed." || true; }
  return 0
}
