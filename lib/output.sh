# ── Output helpers ────────────────────────────────────────────────────────────

BREADCRUMB=""

header() {
  echo "" >&3
  printf '  %s── %s %s──────────────────────────────────────────%s\n' \
    "$C_CYAN" "$1" "$C_DIM" "$C_RESET" >&3
  echo "" >&3
}

ok()       { printf '  %s✓%s %s\n' "$C_GREEN" "$C_RESET" "$1" >&3; }
warn()     { printf '  %s!%s %s\n' "$C_YELLOW" "$C_RESET" "$1" >&3; }
err()      { printf '  %s✗%s %s\n' "$C_RED" "$C_RESET" "$1" >&3; }
dim()      { printf '  %s%s%s\n' "$C_DIM" "$1" "$C_RESET" >&3; }
show_cmd() { printf '  %s$ %s%s\n' "$C_DIM" "$1" "$C_RESET" >&3; }

divider() {
  printf '  %s────────────────────────────────────────────────%s\n' "$C_DIM" "$C_RESET" >&3
}

pause() {
  echo "" >&3
  _redraw_footer
  printf '  %spress any key...%s' "$C_DIM" "$C_RESET" >&3
  read -rsn1
}

drain_stdin() {
  if read -t 0 2>/dev/null; then
    while read -t 0 2>/dev/null; do read -rsn1; done
  fi
  return 0
}

# ── Color-coded kubectl output ────────────────────────────────────────────────

colorize_k8s() {
  local green=$'\033[32m' red=$'\033[31m' yellow=$'\033[33m'
  local dim=$'\033[2m' reset=$'\033[0m'
  sed \
    -e "s/Running/${green}Running${reset}/g" \
    -e "s/Completed/${dim}Completed${reset}/g" \
    -e "s/CrashLoopBackOff/${red}CrashLoopBackOff${reset}/g" \
    -e "s/Error/${red}Error${reset}/g" \
    -e "s/OOMKilled/${red}OOMKilled${reset}/g" \
    -e "s/ImagePullBackOff/${red}ImagePullBackOff${reset}/g" \
    -e "s/ErrImagePull/${red}ErrImagePull${reset}/g" \
    -e "s/CreateContainerConfigError/${red}CreateContainerConfigError${reset}/g" \
    -e "s/Pending/${yellow}Pending${reset}/g" \
    -e "s/Terminating/${yellow}Terminating${reset}/g" \
    -e "s/ContainerCreating/${yellow}ContainerCreating${reset}/g" \
    -e "s/Init:[0-9]*\/[0-9]*/${yellow}&${reset}/g" \
    -e "s/Active/${green}Active${reset}/g" \
    -e "s/ Ready / ${green}Ready${reset} /g" \
    -e "s/NotReady/${red}NotReady${reset}/g" \
    -e "s/SchedulingDisabled/${yellow}SchedulingDisabled${reset}/g" \
    -e "s/ Normal / ${green}Normal${reset} /g" \
    -e "s/ Warning / ${yellow}Warning${reset} /g" \
    -e "s/ True / ${green}True${reset} /g" \
    -e "s/ False / ${red}False${reset} /g"
}
