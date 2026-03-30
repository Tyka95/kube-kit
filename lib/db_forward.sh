# ── DB Port Forward ───────────────────────────────────────────────────────────

DB_PF_POD=""
DB_PF_PID=""
DB_PF_NS=""
_DB_STOP=0

db_forward() {
  header "Database Tunnel"

  # Build target list from config + always offer custom
  local _db_options=()
  if [[ ${#CFG_DB_TARGETS[@]} -gt 0 ]]; then
    for _dbt in "${CFG_DB_TARGETS[@]}"; do
      _db_options+=("$_dbt")
    done
  else
    _db_options+=("Staging Aurora (eu-west-1)|staging-rds-aurora-cluster.cluster-chm826uieyqw.eu-west-1.rds.amazonaws.com|5432")
  fi
  _db_options+=("Custom endpoint")

  local _display_opts=()
  for _dbt in "${_db_options[@]}"; do
    _display_opts+=("${_dbt%%|*}")
  done

  local target
  target=$(printf '%s\n' "${_display_opts[@]}" | gum choose --header "  Database target") || { _redraw_footer; return; }
  clear_content
  header "Database Tunnel"

  # Allow Ctrl+C to cancel prompts and return to menu
  local _db_cancelled=false
  trap '_db_cancelled=true' INT
  trap 'trap - INT' RETURN

  local db_host db_port
  # Look up selected target in options
  local _matched=false
  for _dbt in "${_db_options[@]}"; do
    local _name="${_dbt%%|*}"
    if [[ "$target" == "$_name" && "$_dbt" == *"|"* ]]; then
      local _rest="${_dbt#*|}"
      db_host="${_rest%%|*}"
      db_port="${_rest#*|}"
      dim "Target: $target"
      _matched=true
      break
    fi
  done
  if ! $_matched; then
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

  if ! _validate_port "$local_port"; then
    err "Invalid port number (must be 1-65535)."
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
    # Suppress ^C echo during wait
    local _old_stty
    _old_stty=$(stty -g </dev/tty 2>/dev/null) || true
    stty -echo -isig </dev/tty 2>/dev/null || true

    _DB_STOP=0
    while ((_DB_STOP == 0)) && kill -0 "$DB_PF_PID" 2>/dev/null; do
      # Check for Ctrl+C input (0x03)
      local _byte=""
      _byte=$(perl -e '
        use POSIX qw(tcgetattr tcsetattr TCSANOW);
        open(my $tty, "<", "/dev/tty") or exit 1;
        my $fd = fileno($tty);
        my $old = POSIX::Termios->new; $old->getattr($fd);
        my $raw = POSIX::Termios->new; $raw->getattr($fd);
        $raw->setlflag($raw->getlflag & ~(POSIX::ECHO | POSIX::ICANON | POSIX::ISIG));
        $raw->setcc(POSIX::VMIN, 0);
        $raw->setcc(POSIX::VTIME, 2);  # 0.2s timeout
        $raw->setattr($fd, TCSANOW);
        my $buf;
        my $n = sysread($tty, $buf, 1);
        if (defined $n && $n > 0) {
          printf "%02x", ord($buf);
        }
        $old->setattr($fd, TCSANOW);
      ' 2>/dev/null) || true

      [[ "$_byte" == "03" ]] && _DB_STOP=1

      _tick || true
    done

    # Restore terminal
    stty "$_old_stty" </dev/tty 2>/dev/null || true
    _db_cleanup_display
  else
    err "Port-forward failed (port $local_port may be in use)."
    _db_cleanup_display
  fi
}

# Kill the port-forward process silently (used by INT trap)
_db_kill_fwd() {
  if [[ -n "$DB_PF_PID" ]]; then
    kill "$DB_PF_PID" 2>/dev/null || true
    wait "$DB_PF_PID" 2>/dev/null || true
    DB_PF_PID=""
  fi
}

# Clean up display and socat pod after forward ends
_db_cleanup_display() {
  _db_kill_fwd
  # Smooth transition: just clear content area (keeps header + footer)
  clear_content
  if [[ -n "$DB_PF_POD" ]]; then
    dim "Cleaning up pod $DB_PF_POD..." || true
    kubectl delete pod "$DB_PF_POD" -n "${DB_PF_NS:-default}" --ignore-not-found 2>/dev/null || true
    ok "Deleted pod $DB_PF_POD." || true
    DB_PF_POD=""
  fi
  ok "DB tunnel closed." || true
}

# Full cleanup for EXIT trap (silent kill + pod deletion)
_db_cleanup() {
  if [[ -n "$DB_PF_PID" ]]; then
    kill "$DB_PF_PID" 2>/dev/null || true
    wait "$DB_PF_PID" 2>/dev/null || true
    DB_PF_PID=""
  fi
  if [[ -n "$DB_PF_POD" ]]; then
    kubectl delete pod "$DB_PF_POD" -n "${DB_PF_NS:-default}" --ignore-not-found 2>/dev/null || true
    DB_PF_POD=""
  fi
}
