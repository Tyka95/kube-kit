# ── Screen layout ────────────────────────────────────────────────────────────
# Compact single-line header (k9s-style), status footer, content in between.

# Terminal dimensions — queried via /dev/tty so subshells don't break them
TERM_W=80
TERM_H=24

# Resize detection — polling-based (bash 3.2 loses SIGWINCH during subshells)
_WINCH_FLAG=0

_refresh_term_size() {
  TERM_W=$(stty size </dev/tty 2>/dev/null | awk '{print $2}') || TERM_W=80
  TERM_H=$(stty size </dev/tty 2>/dev/null | awk '{print $1}') || TERM_H=24
  [[ "$TERM_W" =~ ^[0-9]+$ ]] || TERM_W=80
  [[ "$TERM_H" =~ ^[0-9]+$ ]] || TERM_H=24
}

_header_bar() {
  local w=$TERM_W
  # Row 1: full-width top border
  local border=""
  for ((i = 0; i < w; i++)); do border+="─"; done
  printf '%s%s%s\n' "$C_MUTED" "$border" "$C_RESET"
  # Row 2: "  X KubeKit v0.1.5 ────────..."  (X = animated icon placeholder)
  local ver_label="KubeKit v${VERSION}"
  local prefix_len=$((4 + ${#ver_label} + 1))  # "  XX {ver_label} "
  local fill=$((w - prefix_len))
  ((fill < 1)) && fill=1
  local line=""
  for ((i = 0; i < fill; i++)); do line+="─"; done
  # 4 leading spaces: 2 margin + 2 for icon (overwritten by _update_anim)
  printf '    %s%s%s %s%s%s\n' "$C_PRIMARY" "$ver_label" "$C_RESET" "$C_MUTED" "$line" "$C_RESET"
}

_footer_bar() {
  local w=$TERM_W
  local border=""
  for ((i = 0; i < w; i++)); do border+="─"; done

  local _ftr=""
  if [[ -n "$_CTX_CLUSTER" ]]; then
    _ftr+="${C_ACCENT}⎈${C_RESET} ${_CTX_CLUSTER}"
  else
    _ftr+="${C_DANGER}⎈${C_RESET} no cluster"
  fi
  _ftr+="  ${C_MUTED}│${C_RESET}  ${C_ACCENT}⬡${C_RESET} ${_CTX_NS:-default}"
  if [[ -n "$AWS_SESSION_PROFILE" || "$AWS_SESSION_STATUS" != "unknown" ]]; then
    _ftr+="  ${C_MUTED}│${C_RESET}  ${C_ACCENT}☁${C_RESET} ${AWS_SESSION_PROFILE:-<none>}"
    local _ctx_acct _glyph _detail
    _ctx_acct=$(aws_session_context_account)
    case "$AWS_SESSION_STATUS" in
      ok)
        if [[ -n "$_ctx_acct" && -n "$AWS_SESSION_ACCOUNT" && "$_ctx_acct" != "$AWS_SESSION_ACCOUNT" ]]; then
          _glyph="${C_WARN}⚠${C_RESET}"
          _detail="${C_WARN}mismatch ⟶ ${_ctx_acct}${C_RESET}"
        else
          _glyph="${C_SUCCESS}✓${C_RESET}"
          _detail="${C_MUTED}${AWS_SESSION_ACCOUNT}${C_RESET}"
        fi
        ;;
      expired)
        _glyph="${C_DANGER}✗${C_RESET}"
        _detail="${C_DANGER}expired${C_RESET}"
        ;;
      no-aws)
        _glyph="${C_MUTED}–${C_RESET}"
        _detail="${C_MUTED}no aws${C_RESET}"
        ;;
      *)
        _glyph="${C_MUTED}…${C_RESET}"
        _detail="${C_MUTED}validating${C_RESET}"
        ;;
    esac
    _ftr+=" ${_glyph} ${_detail}"
  fi
  printf '%s%s%s\n' "$C_MUTED" "$border" "$C_RESET"
  printf ' %s' "$_ftr"
}

_enter_alt_screen() {
  printf '\033[?1049h' >&3
}

_exit_alt_screen() {
  printf '\033[?1049l' >&3
}

draw_chrome() {
  _refresh_ctx || true
  _refresh_term_size

  # Clear screen and home cursor
  printf '\033[2J\033[H' >&3

  # Hide cursor
  printf '\033[?25l' >&3

  # Header (rows 1-2: border + title)
  _header_bar >&3

  # Footer pinned at bottom (rows TERM_H-1 and TERM_H)
  _redraw_footer

  # Cursor to content area (row 3)
  printf '\033[3;1H' >&3
}

# Full reset for resize — toggle alt screen to flush iTerm2 reflow artifacts
draw_chrome_reset() {
  _exit_alt_screen
  _enter_alt_screen
  draw_chrome
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
  printf '\033[3;1H' >&3
  local i
  for ((i = 3; i <= TERM_H - 2; i++)); do
    printf '\033[2K\033[B' >&3
  done
  _redraw_footer
  printf '\033[3;1H' >&3
}

# ── Menu chooser ──────────────────────────────────────────────────────────────
# Items: "label" or "label|description"
# Returns: 0=selected (label on stdout), 1=back, 2=quit

choose_menu() {
  local title="$1"; shift
  local raw_items=("$@")
  local all_labels=() all_descs=()
  local total=${#raw_items[@]}

  for item in "${raw_items[@]}"; do
    if [[ "$item" == *"|"* ]]; then
      all_labels+=("${item%%|*}")
      all_descs+=("${item#*|}")
    else
      all_labels+=("$item")
      all_descs+=("")
    fi
  done

  # Filter state
  local _filter=""
  local labels=() descs=() _fidx=()  # filtered views
  local count=0 sel=0 scroll=0

  # Rebuild filtered item arrays from _filter
  _apply_filter() {
    labels=(); descs=(); _fidx=()
    local _filt_lower
    _filt_lower=$(echo "$_filter" | tr 'A-Z' 'a-z')
    for ((i = 0; i < total; i++)); do
      if [[ -z "$_filter" ]]; then
        labels+=("${all_labels[$i]}")
        descs+=("${all_descs[$i]}")
        _fidx+=("$i")
      else
        local _lbl_lower
        _lbl_lower=$(echo "${all_labels[$i]}" | tr 'A-Z' 'a-z')
        if [[ "$_lbl_lower" == *"$_filt_lower"* ]]; then
          labels+=("${all_labels[$i]}")
          descs+=("${all_descs[$i]}")
          _fidx+=("$i")
        fi
      fi
    done
    count=${#labels[@]}
    if ((sel >= count)); then sel=0; fi
    scroll=0
  }

  _apply_filter

  local max_visible=$((TERM_H - 9))
  ((max_visible < 3)) && max_visible=3
  local visible=$((count < max_visible ? count : max_visible))

  printf '\033[?25l' >&3

  _draw() {
    ((count == 0)) && visible=0
    ((count > 0 && visible == 0)) && visible=1
    ((visible > count)) && visible=$count
    ((sel < scroll)) && scroll=$sel
    ((sel >= scroll + visible)) && scroll=$((sel - visible + 1))
    ((scroll < 0)) && scroll=0

    {
      # Content starts at row 3 (after 2-line header: border + title)
      printf '\033[3;1H'

      # Breadcrumb + title + optional filter indicator
      printf '\033[K\n'
      if [[ -n "$_filter" ]]; then
        if [[ -n "$BREADCRUMB" ]]; then
          local _title_text="  ⎈  ${BREADCRUMB} › ${title}  / ${_filter} "
          printf '\033[K  %s⎈  %s%s › %s%s  %s/ %s%s\n' \
            "$C_MUTED" "$BREADCRUMB" "$C_RESET" "$C_ACCENT" "$title" "$C_WARN" "$_filter" "$C_RESET"
        else
          local _title_text="  ⎈  ${title}  / ${_filter} "
          printf '\033[K  %s⎈  %s  %s/ %s%s\n' \
            "$C_ACCENT" "$title" "$C_WARN" "$_filter" "$C_RESET"
        fi
      elif [[ -n "$BREADCRUMB" ]]; then
        local _title_text="  ⎈  ${BREADCRUMB} › ${title} "
        printf '\033[K  %s⎈  %s%s › %s%s %s\n' "$C_MUTED" "$BREADCRUMB" "$C_RESET" "$C_ACCENT" "$title" "$C_RESET"
      else
        local _title_text="  ⎈  ${title} "
        printf '\033[K  %s⎈  %s %s\n' "$C_ACCENT" "$title" "$C_RESET"
      fi
      SPINNER_ROW=4
      SPINNER_COL=$((${#_title_text} + 1))
      printf '\033[K\n'

      # Compute max label length for aligned descriptions
      local _max_label=0
      for ((i = 0; i < count; i++)); do
        local _ll=${#labels[$i]}
        ((_ll > _max_label)) && _max_label=$_ll
      done

      # Items — also store text+rows for shimmer
      SHIMMER_TEXTS=()
      SHIMMER_ROWS=()
      SHIMMER_SEL_IDX=-1
      SHIMMER_POS=-1
      SHIMMER_WAIT=0
      SHIMMER_DIR=1

      if ((count == 0)); then
        printf '\033[K  %sno matches%s\n' "$C_MUTED" "$C_RESET"
      fi

      for ((i = 0; i < visible; i++)); do
        local idx=$((scroll + i))
        local item_row=$((6 + i))
        local full_line=""
        local _lbl="${labels[$idx]}"
        local _pad=$((_max_label - ${#_lbl}))
        local _padding=""
        for ((_pi = 0; _pi < _pad; _pi++)); do _padding+=" "; done
        printf '\033[K'
        if ((idx == sel)); then
          full_line="   ❯ ${_lbl}${_padding}"
          local _desc_start=${#full_line}
          [[ -n "${descs[$idx]}" ]] && full_line+="  ${descs[$idx]}"
          printf '  %s ❯ %s%s%s' "$C_PRIMARY" "$_lbl" "$_padding" "$C_RESET"
          if [[ -n "${descs[$idx]}" ]]; then
            printf '  %s%s%s' "$C_ACCENT" "${descs[$idx]}" "$C_RESET"
          fi
          printf '\n'
          SHIMMER_SEL_IDX=$i
          SHIMMER_DESC_START=$_desc_start
        else
          full_line="     ${_lbl}${_padding}"
          [[ -n "${descs[$idx]}" ]] && full_line+="  ${descs[$idx]}"
          printf '  %s   %s%s%s' "$C_MUTED" "$_lbl" "$_padding" "$C_RESET"
          if [[ -n "${descs[$idx]}" ]]; then
            printf '  %s%s%s' "$C_MUTED" "${descs[$idx]}" "$C_RESET"
          fi
          printf '\n'
        fi
        SHIMMER_TEXTS+=("$full_line")
        SHIMMER_ROWS+=("$item_row")
      done

      # Clear remaining lines in content area
      local _clear_from=$((6 + visible))
      local _clear_to=$((TERM_H - 3))
      for ((i = _clear_from; i <= _clear_to; i++)); do
        printf '\033[K\n'
      done

      # Hint bar
      printf '\033[%d;1H' "$((TERM_H - 3))"
      printf '\033[K\n'
      if [[ -n "$_filter" ]]; then
        printf '\033[K  %s↑↓%s navigate  %s→/Enter%s select  %s←/esc%s back  %sBksp%s erase  %stype%s to filter\n' \
          "$C_ACCENT" "$C_MUTED" "$C_ACCENT" "$C_MUTED" "$C_ACCENT" "$C_MUTED" "$C_ACCENT" "$C_MUTED" "$C_ACCENT" "$C_RESET"
      else
        printf '\033[K  %s↑↓%s navigate  %s→/Enter%s select  %s←/esc%s back  %sc%s clear  %sq%s quit  %stype%s to filter\n' \
          "$C_ACCENT" "$C_MUTED" "$C_ACCENT" "$C_MUTED" "$C_ACCENT" "$C_MUTED" "$C_ACCENT" "$C_MUTED" "$C_ACCENT" "$C_MUTED" "$C_ACCENT" "$C_RESET"
      fi

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

  local _prev_w=$TERM_W _prev_h=$TERM_H

  while true; do
    # Poll-based resize detection (every _readkey timeout = ~100ms)
    _refresh_term_size
    if ((TERM_W != _prev_w || TERM_H != _prev_h)); then
      # Debounce: wait for resize to stop
      local _stable=0
      while ((_stable < 2)); do
        sleep 0.15
        local _w=$TERM_W _h=$TERM_H
        _refresh_term_size
        if ((TERM_W == _w && TERM_H == _h)); then
          ((_stable++))
        else
          _stable=0
        fi
      done
      _prev_w=$TERM_W; _prev_h=$TERM_H
      # Invalidate animation state
      SPINNER_ROW=0; SHIMMER_SEL_IDX=-1
      SHIMMER_TEXTS=(); SHIMMER_ROWS=()
      # Fresh alt screen buffer + recalculate layout (full reset for iTerm2)
      draw_chrome_reset
      max_visible=$((TERM_H - 9))
      ((max_visible < 3)) && max_visible=3
      visible=$((count < max_visible ? count : max_visible))
      _draw
    fi
    if ! _readkey; then
      _tick || true
      continue
    fi
    local key="$_HEX"

    # Arrow keys: 1b5b41=up, 1b5b42=down, 1b5b43=right, 1b5b44=left
    case "$key" in
      1b5b41) if ((count > 0)); then if ((sel > 0)); then ((sel--)); else sel=$((count - 1)); fi; _draw; fi; continue ;;
      1b5b42) if ((count > 0)); then if ((sel < count - 1)); then ((sel++)); else sel=0; fi; _draw; fi; continue ;;
      1b5b43) if ((count > 0)); then _select_flash "$((6 + sel - scroll))" "${labels[$sel]}"; echo "${labels[$sel]}"; return 0; fi; continue ;;
      1b5b44) if [[ -n "$_filter" ]]; then _filter=""; _apply_filter; visible=$((count < max_visible ? count : max_visible)); _draw; continue; fi; return 1 ;;
      1b*)    if [[ -n "$_filter" ]]; then _filter=""; _apply_filter; visible=$((count < max_visible ? count : max_visible)); _draw; continue; fi; return 1 ;;
    esac

    # 0a = LF (Enter), 0d = CR
    if [[ "$key" == "0a" || "$key" == "0d" ]]; then
      if ((count > 0)); then
        _select_flash "$((6 + sel - scroll))" "${labels[$sel]}"
        echo "${labels[$sel]}"
        return 0
      fi
      continue
    fi

    # Backspace: 7f or 08
    if [[ "$key" == "7f" || "$key" == "08" ]]; then
      if [[ -n "$_filter" ]]; then
        _filter="${_filter%?}"
        _apply_filter
        visible=$((count < max_visible ? count : max_visible))
        _draw
      fi
      continue
    fi

    # 03 = Ctrl+C
    if [[ "$key" == "03" ]]; then
      return 2
    fi

    # q = 71, c = 63 — commands when filter is empty, filter chars when active
    if [[ "$key" == "71" ]]; then
      if [[ -z "$_filter" ]]; then
        return 2
      fi
    fi
    if [[ "$key" == "63" ]]; then
      if [[ -z "$_filter" ]]; then
        draw_chrome
        _draw
        continue
      fi
    fi

    # 0c = Ctrl+L — always clear
    if [[ "$key" == "0c" ]]; then
      draw_chrome
      _draw
      continue
    fi

    # Printable ASCII (hex 20-7e) → append to filter
    local _dec
    _dec=$((16#$key)) 2>/dev/null || continue
    if ((_dec >= 0x20 && _dec <= 0x7e)); then
      local _ch
      _ch=$(printf "\\x$key")
      _filter+="$_ch"
      _apply_filter
      visible=$((count < max_visible ? count : max_visible))
      _draw
    fi
  done
}
