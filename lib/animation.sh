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
  # Clear icon area (2 cols) to avoid double-width character fragments
  printf '\033[1;%dH  ' "$col" >&3
  local clr
  case $ANIM_PHASE in
    0) clr="$C_WHITE_B" ;;   # flash bright
    1) clr="$C_LCYAN" ;;     # bright cyan
    2) clr="$C_CYAN" ;;      # normal — hold longest
  esac
  printf '\033[1;%dH%s%s%s' "$col" "$clr" "$icon" "$C_RESET" >&3
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
  # Colors: C_CYAN (trail), C_LCYAN (center), C_WHITE_B (lead)
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
      clr="$C_WHITE_B"
      if ((p >= SHIMMER_DESC_START)); then clr="$C_LCYAN"; fi
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
          s_clr="$C_LBLUE"
        elif ((p == p2)); then
          s_clr="$C_WHITE_B"
        elif ((p == p1)); then
          s_clr="$C_CYAN"
        else
          s_clr="$C_LCYAN"
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
        clr="$C_WHITE_B"
        if ((rp >= SHIMMER_DESC_START)); then clr="$C_LCYAN"; fi
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
  printf '\033[%d;%dH%s%s%s' "$SPINNER_ROW" "$SPINNER_COL" "$C_LCYAN" "$frame" "$C_RESET" >&3
  printf '\033[u' >&3
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
    _redraw_footer || true
  fi
  return 0
}
