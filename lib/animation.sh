# ── Animation ────────────────────────────────────────────────────────────────
# Animated icon in the header bar (row 1, col 3)

ANIM_IDX=0

# Animated icon — fast morphing through k8s/cloud/infra symbols
ANIM_ICONS=(
  '⎈' '☸' '⬡' '⬢' '◉' '◈' '✦' '⚡'
  '☁' '▲' '◆' '✧' '⊛' '⏣' '⎔' '⏢'
)
ANIM_PHASE=0

_update_anim() {
  local col=3
  local total=${#ANIM_ICONS[@]}
  local icon_idx=$((ANIM_IDX % total))
  local icon="${ANIM_ICONS[$icon_idx]}"

  printf '\033[s' >&3
  # Clear icon area (2 cols) on row 2 to avoid double-width character fragments
  printf '\033[2;%dH  ' "$col" >&3
  local clr
  case $ANIM_PHASE in
    0) clr="$C_PRIMARY" ;;   # flash bright
    1) clr="$C_ACCENT" ;;     # bright cyan
    2) clr="$C_ACCENT" ;;      # normal — hold longest
  esac
  printf '\033[2;%dH%s%s%s' "$col" "$clr" "$icon" "$C_RESET" >&3
  printf '\033[u' >&3

  ANIM_PHASE=$(( (ANIM_PHASE + 1) % 3 ))
  if ((ANIM_PHASE == 0)); then
    ANIM_IDX=$(( (ANIM_IDX + 1) % total ))
  fi
  return 0
}

# ── Shimmer effect ───────────────────────────────────────────────────────────
# Blue-themed highlight sweeps across menu items like a light reflection

SHIMMER_POS=-1       # current column position of shimmer (-1 = inactive)
SHIMMER_WAIT=0       # ticks to wait before next shimmer cycle
SHIMMER_DIR=1        # 1 = forward, -1 = reverse
declare -a SHIMMER_TEXTS=()
declare -a SHIMMER_ROWS=()
SHIMMER_SEL_IDX=-1
SHIMMER_DESC_START=0 # char index where description (gray) text begins

_shimmer_tick() {
  if ((SHIMMER_SEL_IDX < 0)); then return 0; fi
  if ((${#SHIMMER_TEXTS[@]} == 0)); then return 0; fi

  local row="${SHIMMER_ROWS[$SHIMMER_SEL_IDX]}"
  local text="${SHIMMER_TEXTS[$SHIMMER_SEL_IDX]}"
  local len=${#text}
  if ((len == 0)); then return 0; fi

  # If waiting between cycles, count down
  if ((SHIMMER_POS < 0)); then
    SHIMMER_WAIT=$((SHIMMER_WAIT + 1))
    if ((SHIMMER_WAIT >= 10)); then
      if ((SHIMMER_DIR == 1)); then
        SHIMMER_POS=0
      else
        SHIMMER_POS=$((len - 1))
      fi
      SHIMMER_WAIT=0
    fi
    return 0
  fi

  printf '\033[s' >&3

  # 3-char wide shimmer: trail, center (bright), lead
  # Colors: C_ACCENT (trail), C_ACCENT (center), C_PRIMARY (lead)
  local p0=$((SHIMMER_POS - 2 * SHIMMER_DIR))  # tail to restore
  local p1=$((SHIMMER_POS - SHIMMER_DIR))       # trail
  local p2=$SHIMMER_POS                          # center (brightest)
  local p3=$((SHIMMER_POS + SHIMMER_DIR))        # lead

  local p clr s_clr
  # Restore char that just left the shimmer window
  for p in $p0; do
    if ((p >= 0 && p < len)); then
      local ch="${text:$p:1}"
      local c=$((p + 1))
      clr="$C_PRIMARY"
      if ((p >= SHIMMER_DESC_START)); then clr="$C_ACCENT"; fi
      printf '\033[%d;%dH%s%s%s' "$row" "$c" "$clr" "$ch" "$C_RESET" >&3
    fi
  done

  # Draw 3-char shimmer band
  for p in $p1 $p2 $p3; do
    if ((p >= 0 && p < len)); then
      local ch="${text:$p:1}"
      local c=$((p + 1))
      if [[ "$ch" != " " ]]; then
        if ((p >= SHIMMER_DESC_START)); then
          s_clr="$C_ACCENT"
        elif ((p == p2)); then
          s_clr="$C_PRIMARY"
        elif ((p == p1)); then
          s_clr="$C_ACCENT"
        else
          s_clr="$C_ACCENT"
        fi
        printf '\033[%d;%dH%s%s%s' "$row" "$c" "$s_clr" "$ch" "$C_RESET" >&3
      fi
    fi
  done

  printf '\033[u' >&3

  SHIMMER_POS=$((SHIMMER_POS + SHIMMER_DIR))

  # Check if sweep finished (shimmer fully past text)
  local done=0
  if ((SHIMMER_DIR == 1 && SHIMMER_POS > len + 2)); then done=1; fi
  if ((SHIMMER_DIR == -1 && SHIMMER_POS < -2)); then done=1; fi

  if ((done)); then
    # Restore any remaining shimmer chars
    local rp
    for rp in $((SHIMMER_POS - SHIMMER_DIR)) $((SHIMMER_POS - 2*SHIMMER_DIR)) $((SHIMMER_POS - 3*SHIMMER_DIR)); do
      if ((rp >= 0 && rp < len)); then
        local ch="${text:$rp:1}"
        local c=$((rp + 1))
        clr="$C_PRIMARY"
        if ((rp >= SHIMMER_DESC_START)); then clr="$C_ACCENT"; fi
        printf '\033[s\033[%d;%dH%s%s%s\033[u' "$row" "$c" "$clr" "$ch" "$C_RESET" >&3
      fi
    done
    SHIMMER_DIR=$(( SHIMMER_DIR * -1 ))
    SHIMMER_POS=-1
  fi
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
# Quick bright-white → cyan → dim pulse on the selected item row

_select_flash() {
  local row="$1" text="$2"
  local len=${#text}
  ((len == 0)) && return 0

  # Suppress other animations during flash
  local _saved_spinner=$SPINNER_ROW _saved_shimmer=$SHIMMER_SEL_IDX
  SPINNER_ROW=0; SHIMMER_SEL_IDX=-1

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

  SPINNER_ROW=$_saved_spinner; SHIMMER_SEL_IDX=$_saved_shimmer
  return 0
}

# Periodic tick: icon + spinner + shimmer every call, footer/TTL periodically
_TICK_COUNT=0
_tick() {
  _update_anim || true
  _update_spinner || true
  _shimmer_tick || true
  _TICK_COUNT=$((_TICK_COUNT + 1))
  if ((_TICK_COUNT % 150 == 0)); then
    _update_ttl || true
    # Refresh AWS session every ~150 ticks (~30s of activity). Cheap thanks to
    # the 60s TTL guard inside aws_session_validate.
    if declare -F aws_session_validate &>/dev/null; then
      aws_session_validate || true
    fi
    _redraw_footer || true
  fi
  return 0
}
