# ── Port Forward ──────────────────────────────────────────────────────────────

_validate_port() {
  local p="$1"
  [[ "$p" =~ ^[0-9]+$ ]] || return 1
  ((p >= 1 && p <= 65535)) || return 1
}

PF_PIDS=()
PF_DESCS=()

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

  _pf_show_active
  if [[ -n "${PF_PIDS+x}" ]] && ((${#PF_PIDS[@]} > 0)); then
    warn "Press Ctrl+C to stop all forwards"
    _redraw_footer
    trap '_pf_cleanup; trap - INT' INT
    while true; do
      _pf_purge_dead
      [[ -z "${PF_PIDS+x}" ]] && break
      ((${#PF_PIDS[@]} == 0)) && break
      perl -e 'select(undef,undef,undef,0.2)' 2>/dev/null || sleep 1 || true
      _tick || true
    done
    trap - INT
    _pf_cleanup
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

_pf_cleanup() {
  echo "" >&3 || true
  if [[ -n "${PF_PIDS+x}" ]] && ((${#PF_PIDS[@]} > 0)); then
    for pid in "${PF_PIDS[@]}"; do kill "$pid" 2>/dev/null || true; done
  fi
  PF_PIDS=(); PF_DESCS=()
  warn "All forwards stopped." || true
}
