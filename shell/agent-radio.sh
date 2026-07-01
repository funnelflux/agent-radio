# Source this file from a login shell to use agent-radio helpers.

radio() {
  agent-radio "$@"
}

radio-up() {
  agent-radio up "$@"
}

_agent_radio_repo_slug() {
  local path="${1:-$PWD}"
  basename "$path"
}

_agent_radio_session_id() {
  local tool="$1"
  local repo="${2:-$PWD}"
  local suffix="${3:-}"
  local slug
  slug="$(_agent_radio_repo_slug "$repo")"
  if [ -n "$suffix" ]; then
    printf '%s-%s\n' "$tool" "$suffix"
  else
    printf '%s-%s\n' "$tool" "$slug"
  fi
}

_agent_radio_tmux_run() {
  local id="$1"
  local repo="$2"
  local repo_q id_q cmd_q arg
  shift 2
  printf -v repo_q '%q' "$repo"
  printf -v id_q '%q' "$id"
  cmd_q=""
  for arg in "$@"; do
    local arg_q
    printf -v arg_q '%q' "$arg"
    cmd_q+="${arg_q} "
  done
  radio-up >/dev/null 2>&1 || true
  if tmux has-session -t "$id" 2>/dev/null; then
    tmux attach-session -t "$id"
  else
    tmux new-session -A -s "$id" "cd $repo_q && AGENT_RADIO_ID=$id_q $cmd_q"
  fi
}

codex-new() {
  local repo="${1:-$PWD}"
  local suffix="${2:-}"
  local id
  id="$(_agent_radio_session_id codex "$repo" "$suffix")"
  _agent_radio_tmux_run "$id" "$repo" codex
}

codex-cont() {
  local repo="${1:-$PWD}"
  local suffix="${2:-}"
  local id
  id="$(_agent_radio_session_id codex "$repo" "$suffix")"
  _agent_radio_tmux_run "$id" "$repo" codex
}

cc-new() {
  local repo="${1:-$PWD}"
  local suffix="${2:-}"
  local id
  id="$(_agent_radio_session_id claude "$repo" "$suffix")"
  _agent_radio_tmux_run "$id" "$repo" claude
}

cc-cont() {
  local repo="${1:-$PWD}"
  local suffix="${2:-}"
  local id
  id="$(_agent_radio_session_id claude "$repo" "$suffix")"
  _agent_radio_tmux_run "$id" "$repo" claude
}

tm() {
  case "$1" in
    ""|list)
      tmux list-sessions
      ;;
    open|attach)
      shift
      tmux attach-session -t "$1"
      ;;
    close)
      shift
      tmux kill-session -t "$1"
      ;;
    tree)
      tmux list-windows -a
      ;;
    profile)
      printf 'agent-radio tmux profile\n'
      tmux list-sessions -F '#S'
      ;;
    *)
      tmux "$@"
      ;;
  esac
}
