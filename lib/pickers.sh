# ── Pickers ───────────────────────────────────────────────────────────────────

pick_aws_profile() {
  local profiles
  profiles=$(aws configure list-profiles 2>/dev/null | sort) || true
  [[ -z "$profiles" ]] && { err "No AWS profiles configured." >&2; return 1; }

  local _prof_items=() _prof_line
  while IFS= read -r _prof_line; do
    [[ -z "$_prof_line" ]] && continue
    _prof_items+=("${_prof_line}|profile|")
  done <<< "$profiles"

  PICKER_BINDS=()
  choose_menu "AWS Profile" "${_prof_items[@]}" || return 1
  [[ "$PICKER_RESULT_KIND" != "select" ]] && return 1
  echo "$PICKER_RESULT_VALUE"
}

pick_namespace() {
  local items
  items=$(kubectl get namespaces --no-headers -o custom-columns=":metadata.name,:status.phase" 2>/dev/null | sort) || true
  [[ -z "$items" ]] && { err "No namespaces found." >&2; return 1; }
  local selected
  if ((HAS_FZF)); then
    selected=$(echo "$items" | fzf --header "  Namespace" --ansi \
      --preview "kubectl get pods -n {1} --no-headers 2>/dev/null | head -20" \
      --preview-window=right:50%:wrap) || return 1
    selected=$(echo "$selected" | awk '{print $1}')
  else
    local _ns_items=() _ns_line
    while IFS= read -r _ns_line; do
      [[ -z "$_ns_line" ]] && continue
      local _ns_name _ns_phase
      _ns_name=$(echo "$_ns_line" | awk '{print $1}')
      _ns_phase=$(echo "$_ns_line" | awk '{print $2}')
      _ns_items+=("${_ns_name}|namespace|${_ns_phase}")
    done <<< "$items"
    PICKER_BINDS=()
    choose_menu "Namespace" "${_ns_items[@]}" || return 1
    [[ "$PICKER_RESULT_KIND" != "select" ]] && return 1
    selected="$PICKER_RESULT_VALUE"
  fi
  _save_state "last_namespace" "$selected"
  echo "$selected"
}

pick_pod() {
  local ns
  ns=$(pick_namespace) || return 1
  local items
  items=$(kubectl get pods -n "$ns" --no-headers -o custom-columns=":metadata.name,:status.phase,:status.containerStatuses[0].restartCount" 2>/dev/null) || true
  [[ -z "$items" ]] && { err "No pods in $ns." >&2; return 1; }
  local pod
  if ((HAS_FZF)); then
    pod=$(echo "$items" | fzf --header "  Pod in $ns" --ansi \
      --preview "kubectl describe pod {1} -n $ns 2>/dev/null | head -40" \
      --preview-window=right:50%:wrap) || return 1
    pod=$(echo "$pod" | awk '{print $1}')
  else
    local _pod_items=() _pod_line
    while IFS= read -r _pod_line; do
      [[ -z "$_pod_line" ]] && continue
      local _pod_name _pod_phase _pod_restarts
      _pod_name=$(echo "$_pod_line" | awk '{print $1}')
      _pod_phase=$(echo "$_pod_line" | awk '{print $2}')
      _pod_restarts=$(echo "$_pod_line" | awk '{print $3}')
      _pod_items+=("${_pod_name}|${_pod_phase}|restarts:${_pod_restarts}")
    done <<< "$items"
    PICKER_BINDS=()
    choose_menu "Pod in $ns" "${_pod_items[@]}" || return 1
    [[ "$PICKER_RESULT_KIND" != "select" ]] && return 1
    pod="$PICKER_RESULT_VALUE"
  fi
  echo "$ns/$pod"
}

pick_deployment() {
  local ns
  ns=$(pick_namespace) || return 1
  local items
  items=$(kubectl get deployments -n "$ns" --no-headers -o custom-columns=":metadata.name,:spec.replicas,:status.readyReplicas,:status.updatedReplicas" 2>/dev/null) || true
  [[ -z "$items" ]] && { err "No deployments in $ns." >&2; return 1; }
  local dep
  if ((HAS_FZF)); then
    dep=$(echo "$items" | fzf --header "  Deployment in $ns" --ansi \
      --preview "kubectl describe deployment {1} -n $ns 2>/dev/null | head -40" \
      --preview-window=right:50%:wrap) || return 1
    dep=$(echo "$dep" | awk '{print $1}')
  else
    local _dep_items=() _dep_line
    while IFS= read -r _dep_line; do
      [[ -z "$_dep_line" ]] && continue
      local _dep_name _dep_desired _dep_ready
      _dep_name=$(echo "$_dep_line" | awk '{print $1}')
      _dep_desired=$(echo "$_dep_line" | awk '{print $2}')
      _dep_ready=$(echo "$_dep_line" | awk '{print $3}')
      _dep_items+=("${_dep_name}|deployment|${_dep_ready}/${_dep_desired}")
    done <<< "$items"
    PICKER_BINDS=()
    choose_menu "Deployment in $ns" "${_dep_items[@]}" || return 1
    [[ "$PICKER_RESULT_KIND" != "select" ]] && return 1
    dep="$PICKER_RESULT_VALUE"
  fi
  echo "$ns/$dep"
}
