# ── Configuration ─────────────────────────────────────────────────────────────
# Reads ~/.config/kubekit/config (simple key=value format)
# Supports: aws_regions, default_namespace, default_profile, db_target (multi-value)

CFG_AWS_REGIONS=()
CFG_DEFAULT_NS=""
CFG_DEFAULT_PROFILE=""
CFG_DB_TARGETS=()

_KUBEKIT_CONFIG="${HOME}/.config/kubekit/config"
_KUBEKIT_STATE="${HOME}/.local/state/kubekit/state"

_load_config() {
  CFG_AWS_REGIONS=()
  CFG_DB_TARGETS=()
  CFG_DEFAULT_NS=""
  CFG_DEFAULT_PROFILE=""

  [[ -f "$_KUBEKIT_CONFIG" ]] || return 0

  local line key val
  while IFS= read -r line || [[ -n "$line" ]]; do
    # Skip comments and blank lines
    [[ "$line" =~ ^[[:space:]]*# ]] && continue
    [[ "$line" =~ ^[[:space:]]*$ ]] && continue

    key="${line%%=*}"
    val="${line#*=}"
    # Trim whitespace
    key=$(echo "$key" | tr -d ' ')
    val=$(echo "$val" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')

    case "$key" in
      aws_regions)
        local _raw_regions=()
        IFS=',' read -ra _raw_regions <<< "$val"
        CFG_AWS_REGIONS=()
        for _r in "${_raw_regions[@]}"; do
          _r="${_r#"${_r%%[![:space:]]*}"}"
          _r="${_r%"${_r##*[![:space:]]}"}"
          [[ -n "$_r" ]] && CFG_AWS_REGIONS+=("$_r")
        done
        ;;
      default_namespace)
        CFG_DEFAULT_NS="$val"
        ;;
      default_profile)
        CFG_DEFAULT_PROFILE="$val"
        ;;
      db_target)
        CFG_DB_TARGETS+=("$val")
        ;;
    esac
  done < "$_KUBEKIT_CONFIG"
}

# ── State persistence ────────────────────────────────────────────────────────

_save_state() {
  local key="$1" val="$2"
  local dir
  dir=$(dirname "$_KUBEKIT_STATE")
  [[ -d "$dir" ]] || mkdir -p "$dir"

  if [[ -f "$_KUBEKIT_STATE" ]]; then
    # Remove existing key, then append
    local tmp
    tmp=$(grep -v "^${key}=" "$_KUBEKIT_STATE" 2>/dev/null) || true
    if [[ -n "$tmp" ]]; then
      printf '%s\n' "$tmp" > "$_KUBEKIT_STATE"
    else
      > "$_KUBEKIT_STATE"
    fi
  fi
  echo "${key}=${val}" >> "$_KUBEKIT_STATE"
}

_load_state() {
  local key="$1"
  [[ -f "$_KUBEKIT_STATE" ]] || return 0
  local line
  while IFS= read -r line; do
    if [[ "$line" == "${key}="* ]]; then
      echo "${line#*=}"
      return 0
    fi
  done < "$_KUBEKIT_STATE"
}

_load_config
