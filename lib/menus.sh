# ── Menu engine ───────────────────────────────────────────────────────────────

run_menu() {
  local title="$1"; shift
  local prev_crumb="$BREADCRUMB"
  BREADCRUMB="${BREADCRUMB:+$BREADCRUMB › }$title"

  # Parse items: "Label:function" -> separate arrays
  local item_labels=() item_funcs=()
  for item in "$@"; do
    item_labels+=("${item%%:*}")
    item_funcs+=("${item#*:}")
  done

  draw_chrome

  while true; do
    drain_stdin

    local display=("${item_labels[@]}" "← Back")
    local choice rc=0
    choice=$(choose_menu "$title" "${display[@]}") || rc=$?
    ((rc == 1)) && { BREADCRUMB="$prev_crumb"; return; }
    ((rc == 2)) && exit 0
    [[ "$choice" == "← Back" ]] && { BREADCRUMB="$prev_crumb"; return; }

    for i in "${!item_labels[@]}"; do
      if [[ "${item_labels[$i]}" == "$choice" ]]; then
        drain_stdin
        clear_content
        local arc=0
        ${item_funcs[$i]} || arc=$?
        ((arc == 0)) && pause
        draw_chrome
        break
      fi
    done
  done
  BREADCRUMB="$prev_crumb"
}

# ── Submenu wrappers ──────────────────────────────────────────────────────────

browse_pods()        { list_resource "Pods" pods "-o wide"; }
browse_deploys()     { list_resource "Deployments" deployments "-o wide"; }
view_logs()          { with_pod "View Logs" _view_logs; }
open_shell()         { with_pod "Open Shell" _open_shell; }
inspect_pod()        { with_pod "Inspect Pod" _inspect_pod; }
inspect_deploy()     { with_deployment "Inspect Deployment" _inspect_deploy; }
scale_replicas()     { with_deployment "Scale Replicas" _scale_replicas; }
rolling_restart()    { with_deployment "Rolling Restart" _rolling_restart; }
browse_namespaces()  { list_resource "Namespaces" namespaces; }
browse_services()    { list_resource "Services" services; }
browse_ingresses()   { list_resource "Ingresses" ingress; }
browse_configmaps()  { list_resource "ConfigMaps" configmaps; }
restart_pod()        { with_pod "Restart Pod" _restart_pod; }

# ── Submenus ──────────────────────────────────────────────────────────────────

menu_pods() {
  run_menu "Pods" \
    "Browse Pods:browse_pods" \
    "View Logs:view_logs" \
    "Open Shell:open_shell" \
    "Inspect Pod:inspect_pod" \
    "Restart Pod:restart_pod" \
    "Port Forward:port_forward" \
    "Resource Usage:resource_usage"
}

menu_deployments() {
  run_menu "Deployments" \
    "Browse Deployments:browse_deploys" \
    "Inspect Deployment:inspect_deploy" \
    "Scale Replicas:scale_replicas" \
    "Rolling Restart:rolling_restart"
}

menu_resources() {
  run_menu "Resources" \
    "Namespaces:browse_namespaces" \
    "Services:browse_services" \
    "Ingresses:browse_ingresses" \
    "ConfigMaps:browse_configmaps" \
    "Events:show_events"
}

menu_cluster() {
  run_menu "Cluster" \
    "Current Context:show_context" \
    "Switch Context:switch_context" \
    "Node Status:show_nodes"
}

menu_aws() {
  run_menu "AWS" \
    "SSO Login:sso_login" \
    "Connect EKS Cluster:connect_cluster" \
    "S3 Buckets:list_buckets"
}
