# ── Screen layout ────────────────────────────────────────────────────────────
# Compact single-line header (k9s-style), status footer, content in between.

# Terminal dimensions — queried via /dev/tty so subshells don't break them
TERM_W=80
TERM_H=24

_refresh_term_size() {
  TERM_W=$(stty size </dev/tty 2>/dev/null | awk '{print $2}') || TERM_W=80
  TERM_H=$(stty size </dev/tty 2>/dev/null | awk '{print $1}') || TERM_H=24
  [[ "$TERM_W" =~ ^[0-9]+$ ]] || TERM_W=80
  [[ "$TERM_H" =~ ^[0-9]+$ ]] || TERM_H=24
}

_header_bar() {
  local w=$TERM_W
  # Layout: "  X KubeKit ────────..."  (X = animated icon placeholder, 2 cols wide)
  local prefix_len=13  # "  XX KubeKit "
  local fill=$((w - prefix_len))
  ((fill < 1)) && fill=1
  local line=""
  for ((i = 0; i < fill; i++)); do line+="─"; done
  # 4 leading spaces: 2 margin + 2 for icon (overwritten by _update_anim)
  printf '    %sKubeKit%s %s%s%s\n' "$C_WHITE_B" "$C_RESET" "$C_DIM" "$line" "$C_RESET"
}

_footer_bar() {
  local w=$TERM_W
  local border=""
  for ((i = 0; i < w; i++)); do border+="─"; done

  local _ftr=""
  if [[ -n "$_CTX_CLUSTER" ]]; then
    _ftr+="${C_CYAN}⎈${C_RESET} ${_CTX_CLUSTER}  "
  else
    _ftr+="${C_RED}⎈${C_RESET} no cluster  "
  fi
  _ftr+="${C_CYAN}⬡${C_RESET} ${_CTX_NS:-default}"
  if [[ -n "$_CTX_AWS_PROFILE" ]]; then
    _ftr+="  ${C_CYAN}☁${C_RESET} ${_CTX_AWS_PROFILE}"
    if [[ "$_CTX_AWS_EXPIRY" == "expired" ]]; then
      _ftr+="  ${C_RED}⏱ expired${C_RESET}"
    elif [[ -n "$_CTX_AWS_EXPIRY" ]]; then
      _ftr+="  ${C_GREEN}⏱ ${_CTX_AWS_EXPIRY}${C_RESET}"
    fi
  fi
  printf '%s%s%s\n' "$C_DIM" "$border" "$C_RESET"
  printf ' %s' "$_ftr"
}

draw_chrome() {
  _refresh_ctx
  _refresh_term_size

  # Alt screen + clear + home
  printf '\033[?1049h\033[2J\033[H' >&3

  # Header (row 1 only)
  _header_bar >&3

  # Draw initial animated icon
  _update_anim

  # Footer pinned at bottom (rows TERM_H-1 and TERM_H)
  _redraw_footer

  # Cursor to content area (row 2)
  printf '\033[2;1H' >&3
}

# Redraw just the footer at the bottom of the screen
_redraw_footer() {
  _refresh_term_size
  printf '\033[s' >&3                              # save cursor
  printf '\033[%d;1H\033[K' "$((TERM_H - 1))" >&3 # go to footer row, clear
  printf '\033[%d;1H\033[K' "$TERM_H" >&3          # clear last row too
  printf '\033[%d;1H' "$((TERM_H - 1))" >&3        # back to footer row
  _footer_bar >&3
  printf '\033[u' >&3                              # restore cursor
}

# Clear content area and reposition cursor for action output
clear_content() {
  printf '\033[2;1H' >&3
  local i
  for ((i = 2; i <= TERM_H - 2; i++)); do
    printf '\033[2K\033[B' >&3
  done
  _redraw_footer
  printf '\033[2;1H' >&3
}

# ── Menu chooser ──────────────────────────────────────────────────────────────
# Items: "label" or "label|description"
# Returns: 0=selected (label on stdout), 1=back, 2=quit

choose_menu() {
  local title="$1"; shift
  local raw_items=("$@")
  local labels=() descs=()
  local count=${#raw_items[@]}

  for item in "${raw_items[@]}"; do
    if [[ "$item" == *"|"* ]]; then
      labels+=("${item%%|*}")
      descs+=("${item#*|}")
    else
      labels+=("$item")
      descs+=("")
    fi
  done

  local sel=0
  local visible=$((count < 14 ? count : 14))
  local scroll=0

  printf '\033[?25l' >&3

  _draw() {
    ((sel < scroll)) && scroll=$sel
    ((sel >= scroll + visible)) && scroll=$((sel - visible + 1))

    {
      # Content starts at row 2 (after 1-line header)
      printf '\033[2;1H'

      # Breadcrumb + spinner position
      printf '\033[K\n'
      if [[ -n "$BREADCRUMB" ]]; then
        local _title_text="  ⎈  ${BREADCRUMB} › ${title} "
        printf '\033[K  %s⎈  %s%s › %s%s %s\n' "$C_DIM" "$BREADCRUMB" "$C_RESET" "$C_CYAN_B" "$title" "$C_RESET"
      else
        local _title_text="  ⎈  ${title} "
        printf '\033[K  %s⎈  %s %s\n' "$C_CYAN_B" "$title" "$C_RESET"
      fi
      SPINNER_ROW=3
      SPINNER_COL=$((${#_title_text} + 1))
      printf '\033[K\n'

      # Items — also store text+rows for shimmer
      SHIMMER_TEXTS=()
      SHIMMER_ROWS=()
      SHIMMER_SEL_IDX=-1
      SHIMMER_POS=-1
      SHIMMER_WAIT=0
      SHIMMER_DIR=1
      for ((i = 0; i < visible; i++)); do
        local idx=$((scroll + i))
        local item_row=$((5 + i))
        local full_line=""
        printf '\033[K'
        if ((idx == sel)); then
          full_line="   ❯ ${labels[$idx]}"
          local _desc_start=${#full_line}
          [[ -n "${descs[$idx]}" ]] && full_line+="  ${descs[$idx]}"
          printf '  %s ❯ %s%s' "$C_WHITE_B" "${labels[$idx]}" "$C_RESET"
          if [[ -n "${descs[$idx]}" ]]; then
            printf '  %s%s%s' "$C_LCYAN" "${descs[$idx]}" "$C_RESET"
          fi
          printf '\n'
          SHIMMER_SEL_IDX=$i
          SHIMMER_DESC_START=$_desc_start
        else
          full_line="     ${labels[$idx]}"
          [[ -n "${descs[$idx]}" ]] && full_line+="  ${descs[$idx]}"
          printf '  %s   %s%s' "$C_DIM" "${labels[$idx]}" "$C_RESET"
          if [[ -n "${descs[$idx]}" ]]; then
            printf '  %s%s%s' "$C_DIM" "${descs[$idx]}" "$C_RESET"
          fi
          printf '\n'
        fi
        SHIMMER_TEXTS+=("$full_line")
        SHIMMER_ROWS+=("$item_row")
      done

      # Hint bar
      printf '\033[K\n'
      printf '\033[K  %s↑↓%s navigate  %s→%s select  %s←/esc%s back  %sc%s clear  %sq%s quit\n' \
        "$C_LCYAN" "$C_DIM" "$C_LCYAN" "$C_DIM" "$C_LCYAN" "$C_DIM" "$C_LCYAN" "$C_DIM" "$C_LCYAN" "$C_RESET"

      # Footer pinned at bottom
      printf '\033[%d;1H' "$((TERM_H - 1))"
      _footer_bar
    } >&3
  }

  _draw

  # Set raw input mode once; restore on return
  local _old_stty
  _old_stty=$(stty -g </dev/tty 2>/dev/null) || true
  stty -echo -icanon min 0 time 1 </dev/tty 2>/dev/null || true
  trap 'SPINNER_ROW=0; SHIMMER_SEL_IDX=-1; stty "$_old_stty" </dev/tty 2>/dev/null; printf "\033[?25h" >&3' RETURN

  # _readkey: read up to 4 bytes from tty, return as hex string in _HEX.
  # Reads all available bytes in a single perl call to avoid escape sequence splitting.
  # Returns 1 on timeout (no input).
  _readkey() {
    _HEX=$(perl -e '
      use POSIX qw(tcgetattr tcsetattr TCSANOW);
      open(my $tty, "<", "/dev/tty") or exit 1;
      my $fd = fileno($tty);
      my $old = POSIX::Termios->new; $old->getattr($fd);
      my $raw = POSIX::Termios->new; $raw->getattr($fd);
      $raw->setcc(POSIX::VMIN, 0);
      $raw->setcc(POSIX::VTIME, 1);  # 0.1s timeout
      $raw->setattr($fd, TCSANOW);
      my $buf = "";
      my $n = sysread($tty, $buf, 1);
      if (defined $n && $n > 0) {
        # Got first byte; try to read more (for escape sequences)
        if (ord($buf) == 0x1b) {
          my $more;
          my $n2 = sysread($tty, $more, 3);
          $buf .= $more if defined $n2 && $n2 > 0;
        }
        printf "%s", join("", map { sprintf("%02x", ord($_)) } split(//, $buf));
      }
      $old->setattr($fd, TCSANOW);
    ' 2>/dev/null) || true
    [[ -n "$_HEX" ]]
  }

  while true; do
    if ! _readkey; then
      _tick || true
      continue
    fi
    local key="$_HEX"

    # Arrow keys: 1b5b41=up, 1b5b42=down, 1b5b43=right, 1b5b44=left
    case "$key" in
      1b5b41) if ((sel > 0)); then ((sel--)); else sel=$((count - 1)); fi; _draw; continue ;;
      1b5b42) if ((sel < count - 1)); then ((sel++)); else sel=0; fi; _draw; continue ;;
      1b5b43) echo "${labels[$sel]}"; return 0 ;;
      1b5b44) return 1 ;;
      1b*)    return 1 ;;   # bare ESC or unknown escape
    esac

    # 0a = LF (Enter), 0d = CR
    if [[ "$key" == "0a" || "$key" == "0d" ]]; then
      echo "${labels[$sel]}"
      return 0
    fi

    # 03 = Ctrl+C, 71 = q
    if [[ "$key" == "03" || "$key" == "71" ]]; then
      return 2
    fi

    # 0c = Ctrl+L, 63 = c
    if [[ "$key" == "0c" || "$key" == "63" ]]; then
      draw_chrome
      _draw
    fi
  done
}
