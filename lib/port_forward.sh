# ── Port Forward ──────────────────────────────────────────────────────────────

_validate_port() {
  local p="$1"
  [[ "$p" =~ ^[0-9]+$ ]] || return 1
  ((p >= 1 && p <= 65535)) || return 1
}

PF_PIDS=()
PF_DESCS=()
_PF_STOP=0

port_forward() {
  header "Port Forward"

  while true; do
    local result
    result=$(pick_pod) || return
    local ns="${result%%/*}" pod="${result#*/}"

    local container_ports port=""
    container_ports=$(kubectl get pod "$pod" -n "$ns" \
      -o jsonpath='{range .spec.containers[*]}{.name}:{range .ports[*]}{.containerPort}/{.protocol}{","}{end}{"|"}{end}' 2>/dev/null) || true

    if [[ -n "$container_ports" ]]; then
      local port_options=()
      while IFS='|' read -ra containers; do
        for container in "${containers[@]}"; do
          [[ -z "$container" ]] && continue
          local cname="${container%%:*}" cports="${container#*:}"
          while IFS=',' read -ra pp; do
            for p in "${pp[@]}"; do
              [[ -z "$p" ]] && continue
              port_options+=("${p%%/*}:${p%%/*} (${cname} ${p##*/})")
            done
          done <<< "$cports"
        done
      done <<< "$container_ports"

      if ((${#port_options[@]} > 0)); then
        port_options+=("Custom...")
        local selected
        selected=$(printf '%s\n' "${port_options[@]}" | gum choose --header "  Port") || return
        [[ "$selected" != "Custom..." ]] && port="${selected%% (*}"
      fi
    fi

    [[ -z "$port" ]] && {
      port=$(gum input --placeholder "local:remote (e.g. 3003:3003)" --prompt "  Port: ") || return
    }
    [[ -z "$port" ]] && { err "No port specified."; return; }

    show_cmd "kubectl port-forward pod/$pod $port -n $ns"
    kubectl port-forward "pod/${pod}" "$port" -n "$ns" >/dev/null 2>&1 &
    local pid=$!
    sleep 0.5

    if kill -0 "$pid" 2>/dev/null; then
      PF_PIDS+=("$pid")
      PF_DESCS+=("$ns/$pod $port")
      ok "Forwarding $port from $pod (pid $pid)"
    else
      err "Failed — port may already be in use."
    fi

    gum confirm "Add another forward?" || break
  done

  if [[ -n "${PF_PIDS+x}" ]] && ((${#PF_PIDS[@]} > 0)); then
    # Clean redraw before entering wait mode
    clear_content
    header "Port Forward"
    _pf_show_active
    divider
    warn "Press Ctrl+C to stop all forwards"
    _redraw_footer

    # Suppress ^C echo during wait
    local _old_stty
    _old_stty=$(stty -g </dev/tty 2>/dev/null) || true
    stty -echo -isig </dev/tty 2>/dev/null || true

    # Flag-based: check for Ctrl+C via _readkey instead of signals
    _PF_STOP=0
    while ((_PF_STOP == 0)); do
      _pf_purge_dead
      [[ -z "${PF_PIDS+x}" ]] && break
      ((${#PF_PIDS[@]} == 0)) && break

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

      [[ "$_byte" == "03" ]] && _PF_STOP=1

      _tick || true
    done

    # Restore terminal
    stty "$_old_stty" </dev/tty 2>/dev/null || true

    # Kill remaining forwards
    _pf_kill_all

    # Smooth transition: just clear content area (keeps header + footer)
    clear_content
    warn "All forwards stopped."
  fi
}

_pf_purge_dead() {
  [[ -z "${PF_PIDS+x}" ]] && return 0
  ((${#PF_PIDS[@]} == 0)) && return 0
  local alive_pids=() alive_descs=()
  for i in "${!PF_PIDS[@]}"; do
    if kill -0 "${PF_PIDS[$i]}" 2>/dev/null; then
      alive_pids+=("${PF_PIDS[$i]}")
      alive_descs+=("${PF_DESCS[$i]}")
    fi
  done
  PF_PIDS=("${alive_pids[@]+"${alive_pids[@]}"}")
  PF_DESCS=("${alive_descs[@]+"${alive_descs[@]}"}")
}

_pf_show_active() {
  _pf_purge_dead
  echo "" >&3
  if [[ -n "${PF_PIDS+x}" ]] && ((${#PF_PIDS[@]} > 0)); then
    printf '  %sActive forwards:%s\n' "$C_BOLD" "$C_RESET" >&3
    for i in "${!PF_DESCS[@]}"; do
      ok "${PF_DESCS[$i]} (pid ${PF_PIDS[$i]})"
    done
  else
    warn "No active forwards."
  fi
}

# Kill all port-forward processes silently
_pf_kill_all() {
  if [[ -n "${PF_PIDS+x}" ]] && ((${#PF_PIDS[@]} > 0)); then
    for pid in "${PF_PIDS[@]}"; do kill "$pid" 2>/dev/null || true; done
  fi
  PF_PIDS=(); PF_DESCS=()
}

# Cleanup for EXIT trap in main
_pf_cleanup() {
  _pf_kill_all
}
