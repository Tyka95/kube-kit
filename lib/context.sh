# ── Context info ──────────────────────────────────────────────────────────────

# Cached context — refreshed by _refresh_ctx()
_CTX_CLUSTER=""
_CTX_NS=""
_CTX_AWS_PROFILE=""
_CTX_AWS_EXPIRY=""
_CTX_AWS_EXPIRY_EPOCH=""

_refresh_ctx() {
  _CTX_CLUSTER=$(kubectl config current-context 2>/dev/null) || _CTX_CLUSTER=""
  [[ "$_CTX_CLUSTER" == *"/"* ]] && _CTX_CLUSTER="${_CTX_CLUSTER##*/}"
  _CTX_NS=$(kubectl config view --minify -o jsonpath='{.contexts[0].context.namespace}' 2>/dev/null) || true
  _CTX_NS="${_CTX_NS:-default}"
  _CTX_AWS_PROFILE="${AWS_PROFILE:-${AWS_DEFAULT_PROFILE:-}}"

  # Parse SSO session expiry epoch from newest cache file
  _CTX_AWS_EXPIRY_EPOCH=""
  local cache_file
  cache_file=$(ls -t ~/.aws/sso/cache/*.json 2>/dev/null | head -1) || true
  if [[ -n "$cache_file" ]]; then
    _CTX_AWS_EXPIRY_EPOCH=$(python3 -c "
import json, datetime
try:
    d = json.load(open('$cache_file'))
    exp = d.get('expiresAt','')
    if exp:
        exp_dt = datetime.datetime.fromisoformat(exp.replace('Z','+00:00'))
        print(int(exp_dt.timestamp()))
except: pass
" 2>/dev/null) || true
  fi
  _update_ttl
}

# Cheap TTL update from cached epoch — no python3 call
_update_ttl() {
  _CTX_AWS_EXPIRY=""
  if [[ -n "$_CTX_AWS_EXPIRY_EPOCH" ]]; then
    local now secs
    now=$(date +%s)
    secs=$((_CTX_AWS_EXPIRY_EPOCH - now))
    if ((secs > 0)); then
      _CTX_AWS_EXPIRY="$(( secs / 3600 ))h $(( (secs % 3600) / 60 ))m"
    else
      _CTX_AWS_EXPIRY="expired"
    fi
  fi
}
