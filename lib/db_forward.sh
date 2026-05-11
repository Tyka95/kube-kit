# ── DB Port Forward ───────────────────────────────────────────────────────────

DB_PF_POD=""
DB_PF_PID=""
DB_PF_NS=""
_DB_STOP=0

# In-memory DB discovery cache. Keyed by the validated identity at fetch time.
# Any auth change zeros these via aws_session_context_changed.
DB_CACHE_ACCOUNT=""
DB_CACHE_REGION=""
DB_CACHE_ENTRIES=()
DB_CACHE_TS=0
DB_CACHE_ERROR=""

_DB_CACHE_TTL=60

# Rebuild the picker's _db_options + _menu_items in place. Used by the
# initial render and by the R/P keybindings to re-render after a state change.
_rebuild_db_menu_items() {
  _db_options=()
  _menu_items=()

  local _dbt _name _tag _without_tag
  if [[ ${#CFG_DB_TARGETS[@]} -gt 0 ]]; then
    for _dbt in "${CFG_DB_TARGETS[@]}"; do _db_options+=("$_dbt"); done
  fi
  if [[ ${#DB_CACHE_ENTRIES[@]} -gt 0 ]]; then
    local _new_host _ex _dup
    for _dbt in "${DB_CACHE_ENTRIES[@]}"; do
      _new_host="${_dbt#*|}"
      _new_host="${_new_host%%|*}"
      _dup=false
      if [[ ${#_db_options[@]} -gt 0 ]]; then
        for _ex in "${_db_options[@]}"; do
          [[ "$_ex" == *"|${_new_host}|"* ]] && _dup=true && break
        done
      fi
      $_dup || _db_options+=("$_dbt")
    done
  fi
  _db_options+=("Custom endpoint")

  for _dbt in "${_db_options[@]}"; do
    _name="${_dbt%%|*}"
    if [[ "$_name" == *"["*"]"* ]]; then
      _tag="${_name##*[}"
      _tag="${_tag%]*}"
      _without_tag="${_name% \[*}"
      _menu_items+=("${_without_tag}|discovered|${_tag}")
    elif [[ "$_name" == "Custom endpoint" ]]; then
      _menu_items+=("${_name}|manual|")
    else
      _menu_items+=("${_name}|configured|")
    fi
  done
}

# Populate DB_CACHE_* from AWS RDS for the current session.
# Honors a 60s in-memory TTL keyed on (account, region). Failures set
# DB_CACHE_ERROR and do NOT bump DB_CACHE_TS, so the next call retries.
_discover_db_targets() {
  if [[ "$AWS_SESSION_STATUS" != "ok" ]]; then
    return 0
  fi

  local now
  now=$(date +%s)

  if [[ "$DB_CACHE_ACCOUNT" == "$AWS_SESSION_ACCOUNT" \
     && "$DB_CACHE_REGION"  == "$AWS_SESSION_REGION" \
     && $((now - DB_CACHE_TS)) -lt $_DB_CACHE_TTL ]]; then
    return 0
  fi

  local args=()
  [[ -n "$AWS_SESSION_PROFILE" ]] && args+=(--profile "$AWS_SESSION_PROFILE")
  args+=(--region "$AWS_SESSION_REGION" --output text)
  args+=(--cli-connect-timeout 3 --cli-read-timeout 8)

  # Signal busy state on the chrome border while we're hitting AWS.
  local _prev_chrome="$CHROME_STATE"
  if declare -F set_chrome_state &>/dev/null; then
    set_chrome_state "busy"
    _redraw_footer || true
  fi
  _reset_chrome() {
    if declare -F set_chrome_state &>/dev/null; then
      set_chrome_state "$_prev_chrome"
      _redraw_footer || true
    fi
  }

  local clusters instances err_file rc=0
  err_file=$(mktemp)

  clusters=$(aws rds describe-db-clusters "${args[@]}" \
    --query 'DBClusters[].[DBClusterIdentifier,Endpoint,Port]' 2>"$err_file") || rc=$?
  if (( rc != 0 )); then
    DB_CACHE_ERROR=$(head -n1 "$err_file" 2>/dev/null)
    rm -f "$err_file"
    _reset_chrome
    return 1
  fi

  instances=$(aws rds describe-db-instances "${args[@]}" \
    --query 'DBInstances[?DBClusterIdentifier==`null`].[DBInstanceIdentifier,Endpoint.Address,Endpoint.Port]' 2>"$err_file") || rc=$?
  if (( rc != 0 )); then
    DB_CACHE_ERROR=$(head -n1 "$err_file" 2>/dev/null)
    rm -f "$err_file"
    _reset_chrome
    return 1
  fi
  rm -f "$err_file"

  local id endpoint port entry
  local tag="${AWS_SESSION_PROFILE:-default}"
  DB_CACHE_ENTRIES=()

  if [[ -n "$clusters" ]]; then
    while IFS=$'\t' read -r id endpoint port; do
      [[ -z "$id" || -z "$endpoint" || "$endpoint" == "None" ]] && continue
      entry="${id} (${AWS_SESSION_REGION}) [${tag}]|${endpoint}|${port:-5432}"
      DB_CACHE_ENTRIES+=("$entry")
    done <<< "$clusters"
  fi

  if [[ -n "$instances" ]]; then
    while IFS=$'\t' read -r id endpoint port; do
      [[ -z "$id" || -z "$endpoint" || "$endpoint" == "None" ]] && continue
      entry="${id} (${AWS_SESSION_REGION}) [${tag}]|${endpoint}|${port:-5432}"
      DB_CACHE_ENTRIES+=("$entry")
    done <<< "$instances"
  fi

  DB_CACHE_ACCOUNT="$AWS_SESSION_ACCOUNT"
  DB_CACHE_REGION="$AWS_SESSION_REGION"
  DB_CACHE_TS="$now"
  DB_CACHE_ERROR=""
  _reset_chrome
  return 0
}

db_forward() {
  header "Database Tunnel"
  push_breadcrumb "Database"
  set_keyhints "R refresh" "P profile" "? help"
  draw_chrome

  # Gate: try to ensure an authenticated AWS session before discovery.
  local _can_discover=0
  if aws_session_ensure; then
    _can_discover=1
  fi

  if (( _can_discover )); then
    dim "Discovering databases in account ${AWS_SESSION_ACCOUNT} / region ${AWS_SESSION_REGION}..."
    _discover_db_targets || true
    clear_content
    header "Database Tunnel"
  fi

  # Identity line: show what discovery used (or why it didn't run).
  case "$AWS_SESSION_STATUS" in
    ok)
      local _ctx_acct
      _ctx_acct=$(aws_session_context_account)
      if [[ -n "$_ctx_acct" && "$_ctx_acct" != "$AWS_SESSION_ACCOUNT" ]]; then
        warn "Profile '${AWS_SESSION_PROFILE}' is in account ${AWS_SESSION_ACCOUNT}, but kubectl context targets ${_ctx_acct}."
      else
        dim "Profile: ${AWS_SESSION_PROFILE}  •  Account: ${AWS_SESSION_ACCOUNT}  •  Region: ${AWS_SESSION_REGION}"
      fi
      ;;
    expired) err "AWS session expired. Auto-discovery skipped. Pick a configured target or Custom endpoint." ;;
    no-aws)  dim  "No AWS session available. Showing configured targets only." ;;
    *)       dim  "AWS session unknown. Showing configured targets only." ;;
  esac

  if [[ -n "$DB_CACHE_ERROR" ]]; then
    err "Discovery error: ${DB_CACHE_ERROR}"
  fi

  # Build choose_menu rows: "label|kind|tag". Discovered rows have an
  # embedded "[profile]" tag we strip for the visible label and expose as
  # the meta column. Dedup by hostname is handled inside _rebuild_db_menu_items.
  local _db_options=() _menu_items=()
  _rebuild_db_menu_items

  local target=""
  while :; do
    PICKER_BINDS=("r:refresh" "p:profile")
    if ! choose_menu "Database Tunnel" "${_menu_items[@]}"; then
      pop_breadcrumb
      clear_keyhints
      _redraw_footer
      return
    fi
    case "$PICKER_RESULT_KIND" in
      select)
        target="$PICKER_RESULT_VALUE"
        break
        ;;
      action)
        case "$PICKER_RESULT_VALUE" in
          refresh)
            DB_CACHE_TS=0
            DB_CACHE_ACCOUNT=""
            DB_CACHE_REGION=""
            DB_CACHE_ENTRIES=()
            _discover_db_targets || true
            _rebuild_db_menu_items
            continue
            ;;
          profile)
            local _new_profile
            _new_profile=$(pick_aws_profile) || { draw_chrome; continue; }
            AWS_SESSION_PROFILE="$_new_profile"
            aws_session_context_changed
            aws_session_validate 1 || true
            _discover_db_targets || true
            _rebuild_db_menu_items
            draw_chrome
            continue
            ;;
        esac
        ;;
      *)
        pop_breadcrumb
        clear_keyhints
        _redraw_footer
        return
        ;;
    esac
  done
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
    local _match_name="$_name"
    [[ "$_match_name" == *" ["*"]"* ]] && _match_name="${_match_name% \[*}"
    if [[ "$target" == "$_match_name" && "$_dbt" == *"|"* ]]; then
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
    printf '  %sHost%s %s(Ctrl+C to cancel)%s: ' "$C_ACCENT" "$C_RESET" "$C_MUTED" "$C_RESET" >&3
    read -r db_host < /dev/tty || true
    if $_db_cancelled; then echo "" >&3; pop_breadcrumb; clear_keyhints; return; fi
    printf '  %sDB Port%s [%s5432%s]: ' "$C_ACCENT" "$C_RESET" "$C_MUTED" "$C_RESET" >&3
    read -r db_port < /dev/tty || true
    if $_db_cancelled; then echo "" >&3; pop_breadcrumb; clear_keyhints; return; fi
    db_port="${db_port:-5432}"
  fi
  if [[ -z "$db_host" ]]; then
    err "No host specified."
    pop_breadcrumb
    clear_keyhints
    return
  fi

  local local_port
  drain_stdin
  printf '  %sLocal port%s [%s15432%s] %s(Ctrl+C to cancel)%s: ' "$C_ACCENT" "$C_RESET" "$C_MUTED" "$C_RESET" "$C_MUTED" "$C_RESET" >&3
  read -r local_port < /dev/tty || true
  if $_db_cancelled; then echo "" >&3; pop_breadcrumb; clear_keyhints; return; fi
  local_port="${local_port:-15432}"

  if ! _validate_port "$local_port"; then
    err "Invalid port number (must be 1-65535)."
    pop_breadcrumb
    clear_keyhints
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
    pop_breadcrumb
    clear_keyhints
    return
  fi

  dim "Waiting for pod to be ready..."
  if ! kubectl wait --for=condition=Ready "pod/${pod_name}" -n "$ns" --timeout=60s >&3 2>&3; then
    err "Pod failed to start. Cleaning up..."
    kubectl delete pod "$pod_name" -n "$ns" --ignore-not-found 2>/dev/null
    pop_breadcrumb
    clear_keyhints
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
    printf '  %sConnect with:%s\n' "$C_ACCENT" "$C_RESET" >&3
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
    pop_breadcrumb
    clear_keyhints
    _db_cleanup_display
  else
    err "Port-forward failed (port $local_port may be in use)."
    pop_breadcrumb
    clear_keyhints
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
