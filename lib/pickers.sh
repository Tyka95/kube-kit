# ── Pickers ───────────────────────────────────────────────────────────────────

pick_aws_profile() {
  local profiles
  profiles=$(aws configure list-profiles 2>/dev/null | sort) || true
  [[ -z "$profiles" ]] && { err "No AWS profiles configured." >&2; return 1; }
  echo "$profiles" | gum filter --placeholder "Type to search..." --header "  AWS Profile"
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
    selected=$(echo "$items" | awk '{print $1}' | gum filter --placeholder "Type to search..." --header "  Namespace") || return 1
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
    pod=$(echo "$items" | awk '{print $1}' | gum filter --placeholder "Type to search..." --header "  Pod in $ns") || return 1
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
    dep=$(echo "$items" | awk '{print $1}' | gum filter --placeholder "Type to search..." --header "  Deployment in $ns") || return 1
  fi
  echo "$ns/$dep"
}
