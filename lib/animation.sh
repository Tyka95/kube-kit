# ── Animation ────────────────────────────────────────────────────────────────
# What lives here:
#   - _shimmer_pulse: a one-shot selection-confirmed pulse fired by choose_menu
#     each time the cursor moves. No continuous loops.
#   - _update_spinner: a single-cell braille spinner driven by callers that set
#     SPINNER_ROW/SPINNER_COL while a long-running task runs.
#   - _select_flash: a transient row-flash used by older flows; kept for
#     compatibility, retuned to the v0.2 palette.
#   - _tick: a heartbeat called from the choose_menu input loop. It drives the
#     spinner and triggers a periodic AWS-session revalidation. The old
#     morphing header icon and continuous shimmer loops have been removed.

# ── Selection pulse ──────────────────────────────────────────────────────────
# Fires once when the user moves selection (arrow / page). Sweeps a soft
# C_ACCENT tint across the selected row in 3 frames (~150 ms), then settles
# to plain reverse-video. The full-row reverse selection indicator sits
# underneath, so the pulse is purely confirmation — never the indicator.
#
# Args: $1 = row (1-based), $2 = visible text of the selected row.
_shimmer_pulse() {
  local row="$1" text="$2"
  local len=${#text}
  (( len == 0 )) && return 0
  (( len > 80 )) && len=80

  local frames=("$C_MUTED" "$C_ACCENT" "")
  local delays=(50 60)

  local frame col ch clr
  for ((frame = 0; frame < ${#frames[@]}; frame++)); do
    clr="${frames[$frame]}"
    printf '\033[s' >&3
    for ((col = 1; col <= len; col++)); do
      ch="${text:$((col - 1)):1}"
      if [[ -n "$clr" ]]; then
        printf '\033[%d;%dH%s%s%s%s' "$row" "$col" "$C_REVERSE" "$clr" "$ch" "$C_RESET" >&3
      else
        printf '\033[%d;%dH%s%s%s' "$row" "$col" "$C_REVERSE" "$ch" "$C_RESET" >&3
      fi
    done
    printf '\033[u' >&3
    if (( frame < ${#delays[@]} )); then
      perl -e "select(undef,undef,undef,${delays[$frame]}/1000)" 2>/dev/null || sleep 0.05
    fi
  done
  return 0
}

# ── Braille spinner (thinking dots) ─────────────────────────────────────────
SPINNER_FRAMES=( '⠋' '⠙' '⠹' '⠸' '⠼' '⠴' '⠦' '⠧' '⠇' '⠏' )
SPINNER_IDX=0
SPINNER_ROW=0
SPINNER_COL=0

_update_spinner() {
  ((SPINNER_ROW == 0)) && return 0
  SPINNER_IDX=$(( (SPINNER_IDX + 1) % ${#SPINNER_FRAMES[@]} ))
  local frame="${SPINNER_FRAMES[$SPINNER_IDX]}"
  printf '\033[s' >&3
  printf '\033[%d;%dH%s%s%s' "$SPINNER_ROW" "$SPINNER_COL" "$C_ACCENT" "$frame" "$C_RESET" >&3
  printf '\033[u' >&3
  return 0
}

# ── Selection flash ───────────────────────────────────────────────────────────
# Transient row-flash used by older flows. Retained for compatibility.

_select_flash() {
  local row="$1" text="$2"
  local len=${#text}
  ((len == 0)) && return 0

  local _saved_spinner=$SPINNER_ROW
  SPINNER_ROW=0

  local colors=("$C_PRIMARY" "$C_ACCENT" "$C_ACCENT" "$C_MUTED")
  local delays=(40 50 40 30)

  local phase
  for phase in 0 1 2 3; do
    local clr="${colors[$phase]}"
    printf '\033[s' >&3
    printf '\033[%d;1H\033[2K' "$row" >&3
    printf '  %s ❯ %s%s' "$clr" "$text" "$C_RESET" >&3
    printf '\033[u' >&3
    perl -e "select(undef,undef,undef,${delays[$phase]}/1000)" 2>/dev/null || sleep 0.05
  done

  SPINNER_ROW=$_saved_spinner
  return 0
}

# Periodic heartbeat. Drives the spinner only. Re-validates AWS session every
# ~150 ticks (~30 s of activity). Cheap thanks to the 60 s TTL guard inside
# aws_session_validate.
_TICK_COUNT=0
_tick() {
  _update_spinner || true
  _TICK_COUNT=$((_TICK_COUNT + 1))
  if ((_TICK_COUNT % 150 == 0)); then
    _update_ttl || true
    if declare -F aws_session_validate &>/dev/null; then
      aws_session_validate || true
    fi
    _redraw_footer || true
  fi
  return 0
}
