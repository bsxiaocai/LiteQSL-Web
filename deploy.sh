#!/usr/bin/env bash
# =============================================================================
# LiteQSL-Web 一键部署 / 启动脚本（Linux 服务器）
#
# 直接执行（不带参数）会依次完成：
#   1. 停止现有 LiteQSL-Web 进程
#   2. 访问 GitHub 仓库，检查是否有新代码，有则拉取（未克隆时自动克隆）
#   3. 更新 Python 依赖（创建虚拟环境并安装 requirements.txt）
#   4. 启动 LiteQSL-Web 进程，并保证“除非人为停止，否则不会停止”
#
# 启动方式自动选择：
#   - 以 root 运行且存在 systemd 时：安装为 systemd 服务（Restart=always，进程
#     崩溃会被 systemd 自动拉起，手动停止用 `systemctl stop liteqsl-web`）
#   - 其它情况（普通用户 / 容器）：以“守护循环”方式后台运行，进程崩溃后自动
#     重启，手动停止用 `./deploy.sh stop`
#
# 常用命令：
#   ./deploy.sh          一键部署（停止 -> 更新 -> 装依赖 -> 启动）
#   ./deploy.sh start    停止旧进程并启动
#   ./deploy.sh stop     停止服务
#   ./deploy.sh restart  重启服务
#   ./deploy.sh update   仅拉取更新 + 更新依赖
#   ./deploy.sh status   查看运行状态
#
# 可选环境变量（均有默认值，通常无需设置）：
#   LITEQSL_HOST          监听地址          (默认 0.0.0.0)
#   LITEQSL_PORT          监听端口          (默认 8000)
#   LITEQSL_PYTHON        Python 命令       (默认 python3)
#   LITEQSL_BRANCH        分支              (默认 main)
#   LITEQSL_USER          systemd 运行用户  (默认当前用户)
#   LITEQSL_SECRET_KEY    生产环境 Session 密钥（强烈建议设置）
#   LITEQSL_RESTART_DELAY 崩溃后重启延迟秒数(默认 5)
#
# 首次使用：chmod +x deploy.sh
# =============================================================================

set -o pipefail

# ----------------------------- 配置区 ----------------------------------------
APP_NAME="LiteQSL-Web"
REPO_URL="https://github.com/bsxiaocai/LiteQSL-Web.git"
BRANCH="${LITEQSL_BRANCH:-main}"

# 部署目录 = 本脚本所在目录（即仓库根目录）
DEPLOY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT_PATH="$DEPLOY_DIR/$(basename "${BASH_SOURCE[0]}")"

VENV_DIR="$DEPLOY_DIR/.venv"
RUN_DIR="$DEPLOY_DIR/.run"          # 运行时状态目录（pid / 日志 / 停止标记）
PID_FILE="$RUN_DIR/liteqsl.pid"
STOP_FLAG="$RUN_DIR/stop.flag"
LOG_FILE="$RUN_DIR/liteqsl.log"

HOST="${LITEQSL_HOST:-0.0.0.0}"
PORT="${LITEQSL_PORT:-8000}"
PYTHON="${LITEQSL_PYTHON:-python3}"
SYSTEMD_USER="${LITEQSL_USER:-$(id -un)}"
RESTART_DELAY="${LITEQSL_RESTART_DELAY:-5}"
SERVICE_NAME="liteqsl-web"
# -----------------------------------------------------------------------------

log() { printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"; }
die() { log "错误: $*" >&2; exit 1; }

ensure_runtime_dir() {
  mkdir -p "$RUN_DIR" "$DEPLOY_DIR/data"
}

# 是否可用 systemd 系统级服务（需要 root 且 systemd 正在运行）
has_systemd() {
  command -v systemctl >/dev/null 2>&1 \
    && [ -d /run/systemd/system ] \
    && [ "$(id -u)" -eq 0 ]
}

# uvicorn 启动命令（生产模式，不带 --reload，避免生成 reloader 子进程）
uvicorn_cmd() {
  "$VENV_DIR/bin/uvicorn" app.main:app --host "$HOST" --port "$PORT"
}

# ----------------------------- 停止进程 -------------------------------------
stop_process() {
  ensure_runtime_dir

  # 1) systemd 服务方式
  if has_systemd && [ -f "/etc/systemd/system/$SERVICE_NAME.service" ]; then
    log "停止 systemd 服务 $SERVICE_NAME ..."
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true
    return
  fi

  # 2) 守护循环方式（通过 pid 文件）
  if [ -f "$PID_FILE" ]; then
    local sup_pid child
    sup_pid="$(cat "$PID_FILE" 2>/dev/null || true)"
    if [ -n "$sup_pid" ] && kill -0 "$sup_pid" 2>/dev/null; then
      log "停止守护进程 (pid=$sup_pid) ..."
      # 写入停止标记，让守护循环退出后不再重启
      touch "$STOP_FLAG"
      # 停止守护循环的直接子进程（uvicorn）
      if command -v pgrep >/dev/null 2>&1; then
        for child in $(pgrep -P "$sup_pid" 2>/dev/null || true); do
          kill "$child" 2>/dev/null || true
        done
      fi
      # 等待守护循环自行退出，超时再强制结束
      local i
      for i in {1..10}; do
        kill -0 "$sup_pid" 2>/dev/null || break
        sleep 1
      done
      if kill -0 "$sup_pid" 2>/dev/null; then
        kill -9 "$sup_pid" 2>/dev/null || true
      fi
    fi
    rm -f "$PID_FILE"
    rm -f "$STOP_FLAG"
    return
  fi

  # 3) 兜底：没有 pid 文件（可能是之前手动用 run.py / uvicorn 启动的）
  if command -v pkill >/dev/null 2>&1 && pkill -f "uvicorn app\.main:app" 2>/dev/null; then
    log "已停止手动启动的 uvicorn 进程"
  else
    log "未发现正在运行的 $APP_NAME 进程"
  fi
  rm -f "$STOP_FLAG"
}

# ----------------------------- 更新代码 -------------------------------------
update_code() {
  cd "$DEPLOY_DIR" || die "无法进入目录 $DEPLOY_DIR"

  # 尚未克隆仓库的情况：克隆后同步代码到当前目录
  if [ ! -d .git ]; then
    log "当前目录不是 git 仓库，从 $REPO_URL 克隆（$BRANCH 分支）..."
    local tmp
    tmp="$(mktemp -d)"
    git clone -q -b "$BRANCH" "$REPO_URL" "$tmp" || die "克隆仓库失败，请检查网络与仓库地址"
    if command -v rsync >/dev/null 2>&1; then
      rsync -a \
        --exclude='data/' --exclude='.venv/' --exclude='.run/' --exclude='.git/' \
        "$tmp/" "$DEPLOY_DIR/" || die "同步代码失败"
    else
      log "未找到 rsync，使用 cp 兜底同步 ..."
      shopt -s dotglob
      cp -a "$tmp"/. "$DEPLOY_DIR"/ 2>/dev/null || true
      shopt -u dotglob
    fi
    rm -rf "$tmp"
    # 建立本地 git 仓库，便于之后增量更新
    git init -q
    git remote add origin "$REPO_URL" 2>/dev/null || true
    git fetch -q origin "$BRANCH"
    git checkout -q -B "$BRANCH" "origin/$BRANCH" || true
    log "代码克隆完成，当前 commit: $(git rev-parse HEAD)"
    return
  fi

  log "访问 $REPO_URL 检查 $BRANCH 分支更新 ..."
  git fetch -q --prune --tags origin "$BRANCH" || die "git fetch 失败，请检查网络或仓库地址"

  local local_head remote_head dirty
  local_head="$(git rev-parse HEAD)"
  remote_head="$(git rev-parse "origin/$BRANCH")"

  if [ "$local_head" = "$remote_head" ]; then
    log "代码已是最新 (commit: $local_head)"
    return
  fi

  log "发现新代码: $local_head -> $remote_head"
  dirty=0
  if git status --porcelain --untracked-files=no | grep -q .; then
    dirty=1
  fi

  if [ "$dirty" -eq 1 ]; then
    log "检测到本地修改，先 stash 暂存再拉取 ..."
    git stash push -m "deploy-auto-stash $(date +%s)" -q || die "git stash 失败"
    if git pull --ff-only origin "$BRANCH" -q; then
      log "拉取成功，恢复本地修改 ..."
      git stash pop -q || log "警告: stash pop 冲突，请手动处理（git stash list）"
    else
      log "拉取失败，恢复 stash ..."
      git stash pop -q || true
      die "git pull 失败"
    fi
  else
    git pull --ff-only origin "$BRANCH" -q || die "git pull 失败"
  fi

  log "代码更新完成，当前 commit: $(git rev-parse HEAD)"
}

# ----------------------------- 更新依赖 -------------------------------------
update_deps() {
  cd "$DEPLOY_DIR" || die "无法进入目录 $DEPLOY_DIR"

  if [ ! -d "$VENV_DIR" ]; then
    log "创建虚拟环境 $VENV_DIR ..."
    "$PYTHON" -m venv "$VENV_DIR" || die "创建虚拟环境失败，请确认已安装 $PYTHON 及其 venv 模块"
  fi

  log "安装 / 更新依赖 (requirements.txt) ..."
  "$VENV_DIR/bin/python" -m pip install --upgrade pip -q 2>/dev/null \
    || log "警告: 升级 pip 失败（继续安装依赖）"
  "$VENV_DIR/bin/python" -m pip install --upgrade -r requirements.txt \
    || die "依赖安装失败"
  log "依赖更新完成"
}

# ----------------------------- 启动进程 -------------------------------------
start_systemd() {
  local unit="/etc/systemd/system/$SERVICE_NAME.service"
  if [ ! -f "$unit" ]; then
    log "写入 systemd 服务单元 $unit ..."
    {
      cat <<EOF
[Unit]
Description=LiteQSL-Web QSO/QSL 日志与卡片管理系统
After=network.target

[Service]
Type=simple
User=$SYSTEMD_USER
WorkingDirectory=$DEPLOY_DIR
Environment=PATH=$VENV_DIR/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
Environment=PYTHONDONTWRITEBYTECODE=1
EOF
      [ -n "${LITEQSL_SECRET_KEY:-}" ] && printf 'Environment=SECRET_KEY=%s\n' "$LITEQSL_SECRET_KEY"
      cat <<EOF
ExecStart=$VENV_DIR/bin/uvicorn app.main:app --host $HOST --port $PORT
Restart=always
RestartSec=$RESTART_DELAY
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=$DEPLOY_DIR/data

[Install]
WantedBy=multi-user.target
EOF
    } > "$unit" || die "写入 systemd 单元失败"
    systemctl daemon-reload || die "systemctl daemon-reload 失败"
    systemctl enable "$SERVICE_NAME" -q || die "systemctl enable 失败"
  fi

  log "启动 systemd 服务 $SERVICE_NAME ..."
  systemctl restart "$SERVICE_NAME" || die "systemctl restart 失败"
  log "已通过 systemd 启动，进程崩溃会自动拉起（Restart=always）"
  log "手动停止: systemctl stop $SERVICE_NAME"
}

start_supervised() {
  rm -f "$STOP_FLAG"
  cd "$DEPLOY_DIR"
  log "以守护循环方式启动（进程崩溃后 ${RESTART_DELAY}s 自动重启）..."
  nohup "$SCRIPT_PATH" __supervise >> "$LOG_FILE" 2>&1 &
  echo $! > "$PID_FILE"
  log "已启动，守护进程 pid=$(cat "$PID_FILE")，日志: $LOG_FILE"
  log "手动停止: $0 stop"
}

# 守护循环：反复拉起 uvicorn，仅当检测到停止标记时才退出
supervise_loop() {
  cd "$DEPLOY_DIR"
  local code=0
  while true; do
    uvicorn_cmd
    code=$?
    if [ -f "$STOP_FLAG" ]; then
      rm -f "$STOP_FLAG"
      log "收到停止标记，守护进程退出"
      exit 0
    fi
    log "uvicorn 退出 (code=$code)，${RESTART_DELAY}s 后自动重启 ..."
    sleep "$RESTART_DELAY"
  done
}

start_process() {
  ensure_runtime_dir
  cd "$DEPLOY_DIR"
  if has_systemd; then
    start_systemd
  else
    start_supervised
  fi
}

# ----------------------------- 状态 -----------------------------------------
status() {
  ensure_runtime_dir

  if has_systemd && [ -f "/etc/systemd/system/$SERVICE_NAME.service" ]; then
    systemctl status "$SERVICE_NAME" --no-pager 2>/dev/null || true
  elif [ -f "$PID_FILE" ]; then
    local sup_pid
    sup_pid="$(cat "$PID_FILE" 2>/dev/null || true)"
    if [ -n "$sup_pid" ] && kill -0 "$sup_pid" 2>/dev/null; then
      log "运行中 (守护进程 pid=$sup_pid)"
      command -v pgrep >/dev/null 2>&1 && pgrep -a -P "$sup_pid" 2>/dev/null || true
    else
      log "未运行（存在 pid 文件但进程已退出）"
    fi
  else
    log "未运行"
  fi

  if command -v curl >/dev/null 2>&1; then
    if curl -fsS "http://127.0.0.1:$PORT/health" 2>/dev/null; then
      echo
    else
      log "健康检查未通过（服务可能尚未就绪）"
    fi
  fi
}

usage() {
  cat <<EOF
用法: $0 [命令]

  不带参数 / deploy   一键部署：停止 -> 拉取更新 -> 更新依赖 -> 启动
  start               停止旧进程并启动
  stop                停止服务
  restart             重启服务
  update              仅拉取更新并更新依赖
  status              查看运行状态

环境变量:
  LITEQSL_HOST          监听地址          (默认 0.0.0.0)
  LITEQSL_PORT          监听端口          (默认 8000)
  LITEQSL_PYTHON        Python 命令       (默认 python3)
  LITEQSL_BRANCH        分支              (默认 main)
  LITEQSL_USER          systemd 运行用户  (默认当前用户)
  LITEQSL_SECRET_KEY    生产 Session 密钥
  LITEQSL_RESTART_DELAY 崩溃后重启延迟秒数(默认 5)
EOF
}

# ----------------------------- 主流程 ---------------------------------------
main() {
  case "${1:-}" in
    __supervise) supervise_loop ;;
    start)
      stop_process
      start_process
      ;;
    stop)
      stop_process
      ;;
    restart)
      stop_process
      start_process
      ;;
    update)
      update_code
      update_deps
      ;;
    status)
      status
      ;;
    "" | deploy)
      log "===== 步骤 1/4: 停止现有进程 ====="
      stop_process
      log "===== 步骤 2/4: 检查并更新代码 ====="
      update_code
      log "===== 步骤 3/4: 更新依赖 ====="
      update_deps
      log "===== 步骤 4/4: 启动进程 ====="
      start_process
      log "===== 部署完成 ====="
      sleep 1
      status
      ;;
    *)
      usage
      exit 1
      ;;
  esac
}

main "$@"
