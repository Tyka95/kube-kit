# ── Theme ────────────────────────────────────────────────────────────────────
# Six semantic color tokens. Every color reference in KubeKit must map to one
# of these. Positional names (C_CYAN, C_RED, …) are gone — they tell you the
# color but not the intent. Tokens tell you the intent and let us re-skin later.

# Detect color depth. <8 colors = monochrome fallback (no fg colors, just
# bold + reverse).
_KK_COLORS=$(tput colors 2>/dev/null || echo 0)

if (( _KK_COLORS >= 256 )); then
  # Tokyo Night palette (truecolor / 256-aware terminals).
  C_PRIMARY=$'\033[38;2;224;224;224m'   # #e0e0e0  near-white body text
  C_ACCENT=$'\033[38;2;122;162;247m'    # #7aa2f7  selection / active focus
  C_MUTED=$'\033[38;2;86;95;137m'       # #565f89  secondary metadata
  C_SUCCESS=$'\033[38;2;158;206;106m'   # #9ece6a  ok / validated
  C_WARN=$'\033[38;2;224;175;104m'      # #e0af68  pending / mismatch
  C_DANGER=$'\033[38;2;247;118;142m'    # #f7768e  error / expired
elif (( _KK_COLORS >= 16 )); then
  # 16-color fallback.
  C_PRIMARY=$'\033[37m'
  C_ACCENT=$'\033[94m'
  C_MUTED=$'\033[90m'
  C_SUCCESS=$'\033[92m'
  C_WARN=$'\033[93m'
  C_DANGER=$'\033[91m'
else
  # No-color terminal. Rely on bold/reverse for emphasis.
  C_PRIMARY=""
  C_ACCENT=""
  C_MUTED=""
  C_SUCCESS=""
  C_WARN=""
  C_DANGER=""
fi

# Style modifiers (orthogonal to color tokens).
C_RESET=$'\033[0m'
C_BOLD=$'\033[1m'
C_REVERSE=$'\033[7m'

# ── gum widget styling ───────────────────────────────────────────────────────
# Centralize so every `gum confirm` / `gum input` call automatically follows
# the palette. We intentionally do NOT export styling for `gum choose` /
# `gum filter` — those widgets are being replaced by KubeKit's own choose_menu.
export GUM_CONFIRM_PROMPT_FOREGROUND="${C_PRIMARY}"
export GUM_CONFIRM_SELECTED_BACKGROUND="${C_ACCENT}"
export GUM_CONFIRM_SELECTED_FOREGROUND="${C_PRIMARY}"
export GUM_CONFIRM_UNSELECTED_FOREGROUND="${C_MUTED}"
export GUM_INPUT_CURSOR_FOREGROUND="${C_ACCENT}"
export GUM_INPUT_PROMPT_FOREGROUND="${C_ACCENT}"
export GUM_INPUT_PLACEHOLDER_FOREGROUND="${C_MUTED}"
