# ── Screen layout ────────────────────────────────────────────────────────────
# Compact single-line header (k9s-style), status footer, content in between.

# Terminal dimensions — queried via /dev/tty so subshells don't break them
TERM_W=80
TERM_H=24

# Header content registry: per-screen contextual key hints. Each entry is
# "<key> <action>". Up to 5 entries render right-aligned in the header.
# Set by callers via set_keyhints; cleared on screen exit.
KEYHINTS=()
HEADER_ROWS=3

set_keyhints() {
  KEYHINTS=("$@")
}

clear_keyhints() {
  KEYHINTS=()
}

# Breadcrumb trail. Each entry is a short name shown after "Main ›". Updated
# by push_breadcrumb / pop_breadcrumb at menu boundaries.
BREADCRUMBS=()

push_breadcrumb() {
  BREADCRUMBS+=("$1")
}

pop_breadcrumb() {
  local n=${#BREADCRUMBS[@]}
  (( n > 0 )) && unset 'BREADCRUMBS[n-1]'
  BREADCRUMBS=("${BREADCRUMBS[@]}")
}

clear_breadcrumbs() {
  BREADCRUMBS=()
}

# Picker position string (e.g. "3 of 12"). Set by choose_menu; consumed by
# _footer_bar.
PICKER_POSITION=""

# Chrome state — drives the border color.
# Values: idle | active | busy. set_chrome_state is wired in Task 12.
CHROME_STATE="idle"
CHROME_BORDER_COLOR="$C_MUTED"

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
  local border_color="${CHROME_BORDER_COLOR:-$C_MUTED}"

  # Row 1: ╭─ kubekit ─────…─╮
  local label=" kubekit "
  local left="╭─${label}"
  local right="╮"
  local fill_len=$((w - ${#left} - ${#right}))
  (( fill_len < 0 )) && fill_len=0
  local fill="" i
  for ((i = 0; i < fill_len; i++)); do fill+="─"; done
  printf '%s%s%s%s%s\n' "$border_color" "$left" "$fill" "$right" "$C_RESET"

  # Row 2: status line (k8s / ns / aws segments).
  local status=""
  if [[ -n "$_CTX_CLUSTER" ]]; then
    status+="${C_MUTED}k8s${C_RESET} ${C_PRIMARY}${_CTX_CLUSTER}${C_RESET}"
  else
    status+="${C_MUTED}k8s${C_RESET} ${C_DANGER}no cluster${C_RESET}"
  fi
  status+="  ${C_MUTED}ns${C_RESET} ${C_PRIMARY}${_CTX_NS:-default}${C_RESET}"
  if [[ -n "$AWS_SESSION_PROFILE" || "$AWS_SESSION_STATUS" != "unknown" ]]; then
    status+="  ${C_MUTED}aws${C_RESET} ${C_PRIMARY}${AWS_SESSION_PROFILE:-<none>}${C_RESET}"
    local glyph detail
    case "$AWS_SESSION_STATUS" in
      ok)
        if [[ -n "$AWS_SESSION_CTX_ACCOUNT" && -n "$AWS_SESSION_ACCOUNT" \
           && "$AWS_SESSION_CTX_ACCOUNT" != "$AWS_SESSION_ACCOUNT" ]]; then
          glyph="${C_WARN}⚠${C_RESET}"
          detail="${C_WARN}mismatch ⟶ ${AWS_SESSION_CTX_ACCOUNT}${C_RESET}"
        else
          glyph="${C_SUCCESS}✓${C_RESET}"
          detail="${C_MUTED}${AWS_SESSION_ACCOUNT}${C_RESET}"
        fi ;;
      expired) glyph="${C_DANGER}✗${C_RESET}";  detail="${C_DANGER}expired${C_RESET}" ;;
      no-aws)  glyph="${C_MUTED}–${C_RESET}";   detail="${C_MUTED}no aws${C_RESET}" ;;
      *)       glyph="${C_MUTED}…${C_RESET}";   detail="${C_MUTED}validating${C_RESET}" ;;
    esac
    status+=" ${glyph} ${detail}"
  fi

  # Visible length of status (strip ANSI for padding math).
  local plain
  plain=$(printf '%s' "$status" | sed $'s/\033\\[[0-9;]*m//g')
  local visible_status=${#plain}

  # Right-aligned key hints, up to 5 stacked vertically.
  local hints=()
  local h
  for h in "${KEYHINTS[@]}"; do hints+=("$h"); done
  (( ${#hints[@]} > 5 )) && hints=("${hints[@]:0:5}")

  # Row 2: status on the left, first hint right-aligned.
  local hint0="${hints[0]:-}"
  local pad=$((w - 2 - visible_status - ${#hint0}))
  (( pad < 1 )) && pad=1
  local spaces=""
  for ((i = 0; i < pad; i++)); do spaces+=" "; done
  printf '%s│%s %s%s%s%s %s│%s\n' \
    "$border_color" "$C_RESET" \
    "$status" "$spaces" "$C_MUTED" "$hint0" "$border_color" "$C_RESET"

  # Rows 3+ : remaining hints (right-aligned, blank left).
  local hi hpad hspaces j
  for ((i = 1; i < ${#hints[@]}; i++)); do
    hi="${hints[$i]}"
    hpad=$((w - 4 - ${#hi}))
    (( hpad < 0 )) && hpad=0
    hspaces=""
    for ((j = 0; j < hpad; j++)); do hspaces+=" "; done
    printf '%s│%s%s%s%s %s│%s\n' \
      "$border_color" "$C_RESET" "$hspaces" "$C_MUTED" "$hi" "$border_color" "$C_RESET"
  done

  # Bottom border: ╰─…─╯
  local bottom=""
  for ((i = 0; i < w - 2; i++)); do bottom+="─"; done
  printf '%s╰%s╯%s\n' "$border_color" "$bottom" "$C_RESET"
}

_breadcrumb_row() {
  local trail="${C_MUTED}Main${C_RESET}"
  local b
  for b in "${BREADCRUMBS[@]}"; do
    trail+="${C_MUTED} › ${C_RESET}${C_PRIMARY}${b}${C_RESET}"
  done
  printf ' %s' "$trail"
}

_footer_bar() {
  local w=$TERM_W
  local border_color="${CHROME_BORDER_COLOR:-$C_MUTED}"

  local top="" i
  for ((i = 0; i < w - 2; i++)); do top+="─"; done
  printf '%s╭%s╮%s\n' "$border_color" "$top" "$C_RESET"

  local left="${C_MUTED}/${C_RESET} filter   ${C_MUTED}:${C_RESET} command"
  local right=""
  [[ -n "$PICKER_POSITION" ]] && right+="${C_MUTED}${PICKER_POSITION}${C_RESET}   "
  right+="${C_MUTED}↑↓${C_RESET} select  ${C_MUTED}⏎${C_RESET} go  ${C_MUTED}?${C_RESET} help"

  local plain_left plain_right
  plain_left=$(printf '%s' "$left" | sed $'s/\033\\[[0-9;]*m//g')
  plain_right=$(printf '%s' "$right" | sed $'s/\033\\[[0-9;]*m//g')
  local pad=$((w - 4 - ${#plain_left} - ${#plain_right}))
  (( pad < 1 )) && pad=1
  local spaces=""
  for ((i = 0; i < pad; i++)); do spaces+=" "; done

  printf '%s│%s %s%s%s %s│%s\n' \
    "$border_color" "$C_RESET" "$left" "$spaces" "$right" "$border_color" "$C_RESET"

  local bottom=""
  for ((i = 0; i < w - 2; i++)); do bottom+="─"; done
  printf '%s╰%s╯%s' "$border_color" "$bottom" "$C_RESET"
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

  # Clear screen and home cursor.
  printf '\033[2J\033[H' >&3
  printf '\033[?25l' >&3

  # Header height depends on hint count: 1 top + 1 status + (hints-1) extras + 1 bottom.
  local _hints_len=${#KEYHINTS[@]}
  (( _hints_len > 5 )) && _hints_len=5
  HEADER_ROWS=3
  (( _hints_len > 1 )) && HEADER_ROWS=$((HEADER_ROWS + _hints_len - 1))

  _header_bar >&3
  # Blank line + breadcrumb + blank line below the header block.
  printf '\n' >&3
  _breadcrumb_row >&3
  printf '\n\n' >&3

  _redraw_footer

  # Cursor to content area (right after the breadcrumb's two blank lines).
  printf '\033[%d;1H' "$((HEADER_ROWS + 3))" >&3
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
  printf '\033[s' >&3
  printf '\033[%d;1H\033[K' "$((TERM_H - 2))" >&3
  printf '\033[%d;1H\033[K' "$((TERM_H - 1))" >&3
  printf '\033[%d;1H\033[K' "$TERM_H" >&3
  printf '\033[%d;1H' "$((TERM_H - 2))" >&3
  _footer_bar >&3
  printf '\033[u' >&3
}

# Clear content area and reposition cursor for action output
clear_content() {
  local top=$((HEADER_ROWS + 3))
  local bottom=$((TERM_H - 3))
  printf '\033[%d;1H' "$top" >&3
  local i
  for ((i = top; i <= bottom; i++)); do
    printf '\033[2K\033[B' >&3
  done
  _redraw_footer
  printf '\033[%d;1H' "$top" >&3
}

# Custom keybindings for the next choose_menu invocation. Each entry is
# "<key>:<action>". choose_menu consumes and clears this on entry.
PICKER_BINDS=()

# Result emitted by choose_menu.
#   PICKER_RESULT_KIND=select  + PICKER_RESULT_VALUE=<label>  on Enter
#   PICKER_RESULT_KIND=action  + PICKER_RESULT_VALUE=<action> on custom key
#   PICKER_RESULT_KIND=cancel  + PICKER_RESULT_VALUE=""       on q / esc / non-zero return
PICKER_RESULT_KIND=""
PICKER_RESULT_VALUE=""

# ── Menu chooser ──────────────────────────────────────────────────────────────
# Items: "label", "label|description", or "label|description|metadata"
# Returns: 0=selected, 1=back, 2=quit
# Sets PICKER_RESULT_KIND and PICKER_RESULT_VALUE on all exits.

choose_menu() {
  local title="$1"; shift
  local raw_items=("$@")
  local all_labels=() all_descs=() all_metas=()
  local total=${#raw_items[@]}

  PICKER_RESULT_KIND=""
  PICKER_RESULT_VALUE=""

  for item in "${raw_items[@]}"; do
    if [[ "$item" == *"|"* ]]; then
      local _lbl="${item%%|*}"
      local _rest="${item#*|}"
      if [[ "$_rest" == *"|"* ]]; then
        all_labels+=("$_lbl")
        all_descs+=("${_rest%%|*}")
        all_metas+=("${_rest#*|}")
      else
        all_labels+=("$_lbl")
        all_descs+=("$_rest")
        all_metas+=("")
      fi
    else
      all_labels+=("$item")
      all_descs+=("")
      all_metas+=("")
    fi
  done

  # Filter state
  local _filter=""
  local labels=() descs=() metas=() _fidx=()  # filtered views
  local count=0 sel=0 scroll=0

  # Rebuild filtered item arrays from _filter
  _apply_filter() {
    labels=(); descs=(); metas=(); _fidx=()
    local _filt_lower
    _filt_lower=$(echo "$_filter" | tr 'A-Z' 'a-z')
    for ((i = 0; i < total; i++)); do
      if [[ -z "$_filter" ]]; then
        labels+=("${all_labels[$i]}")
        descs+=("${all_descs[$i]}")
        metas+=("${all_metas[$i]}")
        _fidx+=("$i")
      else
        local _lbl_lower
        _lbl_lower=$(echo "${all_labels[$i]}" | tr 'A-Z' 'a-z')
        if [[ "$_lbl_lower" == *"$_filt_lower"* ]]; then
          labels+=("${all_labels[$i]}")
          descs+=("${all_descs[$i]}")
          metas+=("${all_metas[$i]}")
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

  local row_start=$((HEADER_ROWS + 3))

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

      if ((count == 0)); then
        printf '\033[K  %sno matches%s\n' "$C_MUTED" "$C_RESET"
      fi

      local row=$row_start
      local vis_row idx lbl desc meta
      local lbl_w desc_w meta_w lbl_show desc_show meta_show meta_padded mp row_text
      for ((vis_row = 0; vis_row < visible; vis_row++)); do
        idx=$((scroll + vis_row))
        (( idx >= count )) && break

        lbl="${labels[$idx]}"
        desc="${descs[$idx]}"
        meta="${metas[$idx]:-}"

        # Column budgets: label 25%, desc 45%, meta 20%, gutters ~10%.
        lbl_w=$((TERM_W / 4))
        desc_w=$((TERM_W * 45 / 100))
        meta_w=$((TERM_W / 5))
        (( lbl_w  < 14 )) && lbl_w=14
        (( desc_w <  0 )) && desc_w=0
        (( meta_w <  0 )) && meta_w=0

        lbl_show="${lbl:0:lbl_w}"
        desc_show="${desc:0:desc_w}"
        meta_show="${meta:0:meta_w}"
        while (( ${#lbl_show}  < lbl_w  )); do lbl_show+=" "; done
        while (( ${#desc_show} < desc_w )); do desc_show+=" "; done
        meta_padded=""
        mp=$((meta_w - ${#meta_show}))
        (( mp < 0 )) && mp=0
        while (( ${#meta_padded} < mp )); do meta_padded+=" "; done
        meta_padded+="$meta_show"

        row_text=" ${lbl_show}  ${desc_show}  ${meta_padded}"

        if (( idx == sel )); then
          # Full-row reverse-video selection indicator.
          printf '\033[%d;1H\033[2K%s%s%s' "$row" "$C_REVERSE" "$row_text" "$C_RESET"
        else
          printf '\033[%d;1H\033[2K %s%s%s   %s%s%s   %s%s%s' \
            "$row" \
            "$C_PRIMARY" "$lbl_show"  "$C_RESET" \
            "$C_MUTED"   "$desc_show" "$C_RESET" \
            "$C_MUTED"   "$meta_padded" "$C_RESET"
        fi

        row=$((row + 1))
      done

      # Clear remaining lines in content area
      local _clear_from=$((row_start + visible))
      local _clear_to=$((TERM_H - 3))
      for ((i = _clear_from; i <= _clear_to; i++)); do
        printf '\033[%d;1H\033[2K' "$i"
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

      # Update picker position for footer
      if (( count > 0 )); then
        PICKER_POSITION="$((sel + 1)) of ${count}"
      else
        PICKER_POSITION=""
      fi
    } >&3
    _redraw_footer
  }

  _draw

  # Set raw input mode once; restore on return
  local _old_stty
  _old_stty=$(stty -g </dev/tty 2>/dev/null) || true
  stty -echo -icanon min 0 time 1 </dev/tty 2>/dev/null || true
  trap 'SPINNER_ROW=0; stty "$_old_stty" </dev/tty 2>/dev/null; printf "\033[?25h" >&3' RETURN

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
      SPINNER_ROW=0
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
    local old_sel=$sel

    # Arrow keys: 1b5b41=up, 1b5b42=down, 1b5b43=right, 1b5b44=left
    case "$key" in
      1b5b41)
        if ((count > 0)); then
          if ((sel > 0)); then ((sel--)); else sel=$((count - 1)); fi
          _draw
          if (( sel != old_sel )); then
            _shimmer_pulse "$((row_start + (sel - scroll)))" " ${labels[$sel]}" || true
          fi
        fi
        continue ;;
      1b5b42)
        if ((count > 0)); then
          if ((sel < count - 1)); then ((sel++)); else sel=0; fi
          _draw
          if (( sel != old_sel )); then
            _shimmer_pulse "$((row_start + (sel - scroll)))" " ${labels[$sel]}" || true
          fi
        fi
        continue ;;
      1b5b43)
        if ((count > 0)); then
          PICKER_RESULT_KIND="select"
          PICKER_RESULT_VALUE="${labels[$sel]}"
          echo "${labels[$sel]}"
          return 0
        fi
        continue ;;
      1b5b44)
        if [[ -n "$_filter" ]]; then
          _filter=""; _apply_filter; visible=$((count < max_visible ? count : max_visible)); _draw; continue
        fi
        PICKER_RESULT_KIND="cancel"
        PICKER_RESULT_VALUE=""
        return 1 ;;
      1b*)
        if [[ -n "$_filter" ]]; then
          _filter=""; _apply_filter; visible=$((count < max_visible ? count : max_visible)); _draw; continue
        fi
        PICKER_RESULT_KIND="cancel"
        PICKER_RESULT_VALUE=""
        return 1 ;;
    esac

    # 0a = LF (Enter), 0d = CR
    if [[ "$key" == "0a" || "$key" == "0d" ]]; then
      if ((count > 0)); then
        PICKER_RESULT_KIND="select"
        PICKER_RESULT_VALUE="${labels[$sel]}"
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
      PICKER_RESULT_KIND="cancel"
      PICKER_RESULT_VALUE=""
      return 2
    fi

    # q = 71, c = 63 — commands when filter is empty, filter chars when active
    if [[ "$key" == "71" ]]; then
      if [[ -z "$_filter" ]]; then
        PICKER_RESULT_KIND="cancel"
        PICKER_RESULT_VALUE=""
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

    # Custom keybindings registered by the caller (PICKER_BINDS).
    local _bind _bind_key _bind_action _matched_bind=0
    for _bind in "${PICKER_BINDS[@]}"; do
      _bind_key="${_bind%%:*}"
      _bind_action="${_bind#*:}"
      if [[ "$key" == "$_bind_key" ]]; then
        PICKER_RESULT_KIND="action"
        PICKER_RESULT_VALUE="$_bind_action"
        PICKER_BINDS=()
        printf '\033[?25h' >&3
        if [[ -n "$_old_stty" ]]; then stty "$_old_stty" </dev/tty 2>/dev/null || true; fi
        return 0
      fi
    done

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
