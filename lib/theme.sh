# ── Theme ─────────────────────────────────────────────────────────────────────

C_CYAN=$'\033[36m'
C_CYAN_B=$'\033[1;36m'
C_LCYAN=$'\033[96m'        # Bright/light cyan
C_WHITE_B=$'\033[1;37m'    # Bright white
C_LBLUE=$'\033[38;5;117m'  # Light blue (256-color)
C_GREEN=$'\033[32m'
C_YELLOW=$'\033[33m'
C_RED=$'\033[31m'
C_DIM=$'\033[2m'
C_BOLD=$'\033[1m'
C_RESET=$'\033[0m'

# Gum theme — override default pink with cyan
export GUM_FILTER_INDICATOR_FOREGROUND="6"
export GUM_FILTER_MATCH_FOREGROUND="6"
export GUM_FILTER_PROMPT_FOREGROUND="6"
export GUM_FILTER_CURSOR_FOREGROUND="6"
export GUM_CHOOSE_CURSOR_FOREGROUND="6"
export GUM_CHOOSE_SELECTED_FOREGROUND="6"
export GUM_CONFIRM_SELECTED_FOREGROUND="6"
export GUM_CONFIRM_PROMPT_FOREGROUND="6"
export GUM_INPUT_CURSOR_FOREGROUND="6"
export GUM_INPUT_PROMPT_FOREGROUND="6"
export GUM_SPIN_SPINNER_FOREGROUND="6"
