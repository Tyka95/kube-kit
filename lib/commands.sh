# ── Command palette ──────────────────────────────────────────────────────────
# `:` from any picker activates a single-line command input. Commands are
# registered globally with register_command and dispatched by run_command.

COMMAND_NAMES=()
COMMAND_DESCS=()
COMMAND_FNS=()
COMMAND_RESULT=""

register_command() {
  local name="$1" desc="$2" fn="$3"
  COMMAND_NAMES+=("$name")
  COMMAND_DESCS+=("$desc")
  COMMAND_FNS+=("$fn")
}

_cmd_lookup() {
  local q="$1" i
  for ((i = 0; i < ${#COMMAND_NAMES[@]}; i++)); do
    if [[ "${COMMAND_NAMES[$i]}" == "$q" ]]; then
      printf '%s' "${COMMAND_FNS[$i]}"
      return 0
    fi
  done
  return 1
}

run_command() {
  local input="$1"
  local name="${input%% *}"
  local rest=""
  [[ "$input" == *" "* ]] && rest="${input#* }"
  local fn
  fn=$(_cmd_lookup "$name") || { COMMAND_RESULT="unknown command: $name"; return 1; }
  COMMAND_RESULT=""
  "$fn" "$rest"
}

# Built-ins.
_cmd_quit() { COMMAND_RESULT="__quit__"; }
_cmd_help() { COMMAND_RESULT="__help__"; }

_cmd_ns() {
  local ns="$1"
  [[ -z "$ns" ]] && { COMMAND_RESULT="ns: missing argument"; return 1; }
  if kubectl config set-context --current --namespace="$ns" &>/dev/null; then
    if declare -F _refresh_ctx &>/dev/null; then _refresh_ctx; fi
    if declare -F _redraw_footer &>/dev/null; then _redraw_footer; fi
    if declare -F draw_chrome &>/dev/null; then draw_chrome; fi
    COMMAND_RESULT="namespace → $ns"
  else
    COMMAND_RESULT="ns: failed"
    return 1
  fi
}

_cmd_context() {
  local ctx="$1"
  [[ -z "$ctx" ]] && { COMMAND_RESULT="context: missing argument"; return 1; }
  if kubectl config use-context "$ctx" &>/dev/null; then
    if declare -F aws_session_context_changed &>/dev/null; then
      aws_session_context_changed
    fi
    if declare -F _refresh_ctx &>/dev/null; then _refresh_ctx; fi
    if declare -F aws_session_validate &>/dev/null; then
      aws_session_validate 1 || true
    fi
    if declare -F draw_chrome &>/dev/null; then draw_chrome; fi
    COMMAND_RESULT="context → $ctx"
  else
    COMMAND_RESULT="context: failed"
    return 1
  fi
}

_cmd_profile() {
  local p="$1"
  [[ -z "$p" ]] && { COMMAND_RESULT="profile: missing argument"; return 1; }
  AWS_SESSION_PROFILE="$p"
  if declare -F aws_session_context_changed &>/dev/null; then
    aws_session_context_changed
  fi
  if declare -F aws_session_validate &>/dev/null; then
    aws_session_validate 1 || true
  fi
  if declare -F draw_chrome &>/dev/null; then draw_chrome; fi
  COMMAND_RESULT="profile → $p"
}

register_command "q"       "quit"               _cmd_quit
register_command "quit"    "quit"               _cmd_quit
register_command "help"    "show help overlay"  _cmd_help
register_command "ns"      "set namespace"      _cmd_ns
register_command "context" "switch kube ctx"    _cmd_context
register_command "profile" "switch aws profile" _cmd_profile
