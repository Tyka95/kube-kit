#!/usr/bin/env bash
# pipefail is useful for catching subshell failures; -u catches unbound vars.
# -e is intentionally OFF: this is an interactive TUI with many user-facing
# paths that legitimately fail (gum cancel, kubectl unreachable, stty edge
# cases) and using `cmd || true` defensively everywhere makes the code
# brittle. Failures should surface as user-visible errors, not silent exits.
set -uo pipefail

VERSION="0.1.10" # x-release-please-version

# ── Version flag (handle before anything else) ────────────────────────────────
case "${1:-}" in
  --version|-v) echo "kubekit $VERSION"; exit 0 ;;
esac

# ── Display file descriptor ──────────────────────────────────────────────────
# fd 3 = direct to terminal, never captured by $() subshells
exec 3>/dev/tty

# ── Dependency check ────────────────────────────────────────────────────────
if ! command -v gum &>/dev/null; then
  echo "ERROR: 'gum' is required but not installed."
  echo "Install with: brew install gum"
  exit 1
fi

HAS_FZF=0
command -v fzf &>/dev/null && HAS_FZF=1

# ── Source library modules ──────────────────────────────────────────────────
_self="${BASH_SOURCE[0]}"
# Resolve symlinks (works on macOS without GNU readlink)
while [ -L "$_self" ]; do
  _dir="$(cd "$(dirname "$_self")" && pwd)"
  _self="$(readlink "$_self")"
  [[ "$_self" != /* ]] && _self="$_dir/$_self"
done
KUBE_LIB="$(cd "$(dirname "$_self")" && pwd)/lib"
unset _self _dir

source "$KUBE_LIB/config.sh"
source "$KUBE_LIB/aws_session.sh"
source "$KUBE_LIB/theme.sh"
source "$KUBE_LIB/output.sh"
source "$KUBE_LIB/context.sh"
source "$KUBE_LIB/animation.sh"
source "$KUBE_LIB/ui.sh"
source "$KUBE_LIB/pickers.sh"
source "$KUBE_LIB/session.sh"
source "$KUBE_LIB/actions_pods.sh"
source "$KUBE_LIB/actions_deploy.sh"
source "$KUBE_LIB/actions_cluster.sh"
source "$KUBE_LIB/actions_aws.sh"
source "$KUBE_LIB/port_forward.sh"
source "$KUBE_LIB/db_forward.sh"
source "$KUBE_LIB/menus.sh"
source "$KUBE_LIB/commands.sh"

# ── Main ──────────────────────────────────────────────────────────────────────

main() {
  trap '_db_cleanup 2>/dev/null; _pf_cleanup 2>/dev/null; _exit_alt_screen' EXIT

  # Restore last namespace from state (or config default)
  local _restored_ns
  _restored_ns=$(_load_state "last_namespace") || true
  [[ -z "$_restored_ns" ]] && _restored_ns="$CFG_DEFAULT_NS"
  if [[ -n "$_restored_ns" ]]; then
    kubectl config set-context --current --namespace="$_restored_ns" &>/dev/null || true
  fi

  # Kick off the first AWS validate so the footer becomes accurate within seconds.
  aws_session_validate || true

  _enter_alt_screen
  draw_chrome

  while true; do
    drain_stdin
    BREADCRUMB=""

    local rc=0
    choose_menu "Main Menu" \
      "Pods|list · logs · shell · inspect" \
      "Deployments|browse · scale · restart" \
      "Resources|namespaces · services · ingress" \
      "Cluster|context · nodes" \
      "Database|tunnel via socat pod" \
      "AWS|sso · eks · s3" \
      "Exit" || rc=$?
    # rc=1 = ESC / q (cancel). At the root menu we treat this as a no-op
    # — quit is only via Ctrl+C (rc=2), the explicit Exit item, or :q.
    ((rc >= 2)) && break
    (( rc == 1 )) && continue

    if [[ "$PICKER_RESULT_KIND" == "action" ]]; then
      case "$PICKER_RESULT_VALUE" in
        __quit__) break ;;
        __help__) show_help_overlay ;;
      esac
      continue
    fi

    [[ "$PICKER_RESULT_KIND" != "select" ]] && continue
    local choice="$PICKER_RESULT_VALUE"

    case "$choice" in
      Pods)        menu_pods ;;
      Deployments) menu_deployments ;;
      Resources)   menu_resources ;;
      Cluster)     menu_cluster ;;
      Database)    clear_content; db_forward; pause; ;;
      AWS)         menu_aws ;;
      Exit)        break ;;
    esac
    draw_chrome
  done

  _exit_alt_screen
  ok "Bye!"
}

main
