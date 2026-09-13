#!/usr/bin/env bash
# =============================================================================
# LiteQSL-Web v2 部署 / 启动脚本（Go 单体程序版，无需 Python/pip/venv）
#
# 不带参数执行会依次完成：
#   1. 停止现有 LiteQSL-Web 进程
#   2. 准备程序（本地已有则直接用；否则从 Releases 下载发布包；再否则用 Go 从源码构建）
#   3. 准备运行布局（static/、config.yaml、data/）
#   4. 启动服务，并保证“除非人为停止，否则不会停止”
#
# 启动方式自动选择：
#   - root 且存在 systemd：安装为 systemd 服务（Restart=always，开机自启）
#   - 其它情况（普通用户 / 容器）：守护循环后台运行，崩溃自动重启
#
# 常用命令：
#   ./deploy.sh                  一键部署（停止 -> 获取程序 -> 准备布局 -> 启动）
#   ./deploy.sh start            停止旧进程并启动
#   ./deploy.sh stop             停止服务
#   ./deploy.sh restart          重启服务
#   ./deploy.sh update           仅更新程序（下载/构建）并重启
#   ./deploy.sh build            仅用 Go 从源码构建二进制
#   ./deploy.sh status           查看运行状态与健康检查
#   ./deploy.sh install-service  仅安装 systemd 服务
#
# 可选环境变量：
#   LITEQSL_RELEASE_URL   发布下载基地址（默认指向本仓库 Releases 最新版，直接下载发布包）
#                         例: https://github.com/bsxiaocai/LiteQSL-Web/releases/latest/download
#   LITEQSL_HOST          监听地址（写入 config.yaml 时的默认值，默认 0.0.0.0）
#   LITEQSL_PORT          监听端口（默认 8000）
#   LITEQSL_USER          systemd 运行用户（默认当前用户）
#   LITEQSL_SECRET_KEY    会话密钥（写入 systemd 环境变量）
#   LITEQSL_RESTART_DELAY 崩溃后重启延迟秒数（默认 5）
#   GO                    go 命令（默认 go）
#
# 首次使用：chmod +x deploy.sh
# =============================================================================

set -o pipefail

# ----------------------------- 配置区 ----------------------------------------
APP_NAME="LiteQSL-Web"
REPO_URL="${LITEQSL_REPO_URL:-https://github.com/bsxiaocai/LiteQSL-Web.git}"
BRANCH="${LITEQSL_BRANCH:-main}"

# 部署目录 = 本脚本所在目录
DEPLOY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT_PATH="$DEPLOY_DIR/$(basename "${BASH_SOURCE[0]}")"

# Windows(Git Bash/MSYS2) 下二进制带 .exe 后缀
case "$(uname -s 2>/dev/null | tr '[:upper:]' '[:lower:]')" in
  mingw*|msys*|cygwin*) BINARY_NAME="liteqsl.exe" ;;
  *)                    BINARY_NAME="liteqsl" ;;
esac
BINARY="$DEPLOY_DIR/$BINARY_NAME"
STATIC_DIR="$DEPLOY_DIR/static"
CONFIG_FILE="$DEPLOY_DIR/config.yaml"
CONFIG_EXAMPLE="$DEPLOY_DIR/config.example.yaml"
DATA_DIR="$DEPLOY_DIR/data"

RUN_DIR="$DEPLOY_DIR/.run"
PID_FILE="$RUN_DIR/liteqsl.pid"
CHILD_PID_FILE="$RUN_DIR/liteqsl.child.pid"
STOP_FLAG="$RUN_DIR/stop.flag"
LOG_FILE="$RUN_DIR/liteqsl.log"

HOST="${LITEQSL_HOST:-0.0.0.0}"
PORT="${LITEQSL_PORT:-8000}"
SYSTEMD_USER="${LITEQSL_USER:-$(id -un 2>/dev/null || echo nobody)}"
RESTART_DELAY="${LITEQSL_RESTART_DELAY:-5}"
RELEASE_URL="${LITEQSL_RELEASE_URL:-https://github.com/bsxiaocai/LiteQSL-Web/releases/latest/download}"
GO_CMD="${GO:-go}"
SERVICE_NAME="liteqsl"
# -----------------------------------------------------------------------------

log() { printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"; }
die() { log "错误: $*" >&2; exit 1; }

ensure_runtime_dir() { mkdir -p "$RUN_DIR" "$DATA_DIR"; }

# 是否可用 systemd 系统级服务（需要 root 且 systemd 正在运行）
has_systemd() {
  command -v systemctl >/dev/null 2>&1 \
    && [ -d /run/systemd/system ] \
    && [ "$(id -u)" -eq 0 ]
}

# 识别平台（os arch），用于选择/下载对应二进制
detect_platform() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    linux)  os="linux" ;;
    darwin) os="darwin" ;;
    mingw*|msys*|cygwin*) os="windows" ;;
    *) die "不支持的系统: $os" ;;
  esac
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    armv7l|armv6l) arch="arm" ;;
    *) die "不支持的架构: $arch" ;;
  esac
  echo "$os $arch"
}

# ----------------------------- 准备布局 -------------------------------------
ensure_layout() {
  ensure_runtime_dir

  [ -d "$STATIC_DIR" ] || die "缺少 static/ 目录（请在发布包根目录运行本脚本，或先放置前端资源）"

  if [ ! -f "$CONFIG_FILE" ]; then
    if [ -f "$CONFIG_EXAMPLE" ]; then
      cp "$CONFIG_EXAMPLE" "$CONFIG_FILE"
      log "已根据 config.example.yaml 生成 config.yaml（请按需修改）"
    else
      log "未找到 config.yaml / config.example.yaml，将使用内置默认配置"
    fi
  fi
}

# ----------------------------- 获取程序 -------------------------------------
# 优先级：已存在的二进制 > 从发布地址下载发布包并解压 > 用 Go 从源码构建
fetch_binary() {
  local force="${1:-0}"
  local os arch ext pkg url tmpdir
  read -r os arch <<< "$(detect_platform)"
  ext=""
  [ "$os" = "windows" ] && ext=".exe"

  if [ "$force" -eq 0 ] && [ -x "$BINARY" ]; then
    log "使用已有二进制: $BINARY"
    return 0
  fi

  if [ -n "$RELEASE_URL" ]; then
    # 发布包内同时包含二进制与 static/，因此下载压缩包而非裸二进制
    pkg="liteqsl-${os}-${arch}.tar.gz"
    url="${RELEASE_URL%/}/${pkg}"
    tmpdir="$(mktemp -d)"
    log "下载发布包: $url"
    if command -v curl >/dev/null 2>&1; then
      curl -fSL --retry 3 -o "$tmpdir/$pkg" "$url" || { rm -rf "$tmpdir"; die "下载失败: $url"; }
    elif command -v wget >/dev/null 2>&1; then
      wget -O "$tmpdir/$pkg" "$url" || { rm -rf "$tmpdir"; die "下载失败: $url"; }
    else
      rm -rf "$tmpdir"
      die "需要 curl 或 wget 才能下载发布包"
    fi

    log "解压发布包 ..."
    mkdir -p "$tmpdir/extract"
    tar -xzf "$tmpdir/$pkg" -C "$tmpdir/extract" || { rm -rf "$tmpdir"; die "解压失败: $pkg"; }
    [ -f "$tmpdir/extract/liteqsl${ext}" ] || { rm -rf "$tmpdir"; die "发布包内容异常，未找到 liteqsl${ext}"; }

    # 安装二进制与前端资源（覆盖旧版本）；其余文件仅在缺失时补齐，避免覆盖用户配置
    cp -f "$tmpdir/extract/liteqsl${ext}" "$BINARY"
    chmod +x "$BINARY" 2>/dev/null || true
    if [ -d "$tmpdir/extract/static" ]; then
      rm -rf "$STATIC_DIR"
      cp -r "$tmpdir/extract/static" "$STATIC_DIR"
    fi
    [ -f "$CONFIG_EXAMPLE" ] || cp -f "$tmpdir/extract/config.example.yaml" "$CONFIG_EXAMPLE" 2>/dev/null || true
    [ -f "$DEPLOY_DIR/deploy.sh" ] || cp -f "$tmpdir/extract/deploy.sh" "$DEPLOY_DIR/deploy.sh" 2>/dev/null || true
    if [ ! -d "$DEPLOY_DIR/deploy" ] && [ -d "$tmpdir/extract/deploy" ]; then
      cp -r "$tmpdir/extract/deploy" "$DEPLOY_DIR/deploy"
    fi
    rm -rf "$tmpdir"

    log "已安装: $BINARY（含 static/）"
    return 0
  fi

  # 从源码构建
  if command -v "$GO_CMD" >/dev/null 2>&1; then
    build_binary
    return 0
  fi

  die "未找到可用程序。请设置 LITEQSL_RELEASE_URL 下载发布包，或安装 Go 后从源码构建"
}

# 用 Go 从源码构建
build_binary() {
  [ -f "$DEPLOY_DIR/go.mod" ] || die "当前目录不是源码仓库，无法构建（缺少 go.mod）"
  command -v "$GO_CMD" >/dev/null 2>&1 || die "未找到 go 命令，请安装 Go 1.26+"
  log "使用 Go 从源码构建 ..."
  ( cd "$DEPLOY_DIR" && CGO_ENABLED=0 "$GO_CMD" build -trimpath -ldflags "-s -w" -o "$BINARY" ./cmd/liteqsl ) \
    || die "构建失败"
  log "构建完成: $BINARY"
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

    # 先结束守护循环记录的子进程（真正的服务进程）——不依赖 pgrep，
    # 以兼容 Git Bash / MSYS 等无法用 pgrep -P 追踪 Windows 子进程的环境。
    if [ -f "$CHILD_PID_FILE" ]; then
      child="$(cat "$CHILD_PID_FILE" 2>/dev/null || true)"
      if [ -n "$child" ] && kill -0 "$child" 2>/dev/null; then
        log "停止服务进程 (pid=$child) ..."
        kill "$child" 2>/dev/null || true
      fi
    fi

    if [ -n "$sup_pid" ] && kill -0 "$sup_pid" 2>/dev/null; then
      log "停止守护进程 (pid=$sup_pid) ..."
      touch "$STOP_FLAG"
      if command -v pgrep >/dev/null 2>&1; then
        for child in $(pgrep -P "$sup_pid" 2>/dev/null || true); do
          kill "$child" 2>/dev/null || true
        done
      fi
      local i
      for i in $(seq 1 10); do
        kill -0 "$sup_pid" 2>/dev/null || break
        sleep 1
      done
      kill -0 "$sup_pid" 2>/dev/null && kill -9 "$sup_pid" 2>/dev/null || true
    fi
    rm -f "$PID_FILE" "$CHILD_PID_FILE" "$STOP_FLAG"

    # 兜底：确保没有残留的服务进程
    stop_stray_binary
    return
  fi

  # 3) 兜底：没有 pid 文件
  if ! stop_stray_binary; then
    log "未发现正在运行的 $APP_NAME 进程"
  fi
  rm -f "$STOP_FLAG"
}

# 兜底结束仍然运行的本程序进程（按命令行匹配）
stop_stray_binary() {
  local killed=0
  if command -v pkill >/dev/null 2>&1; then
    if pkill -f "$BINARY_NAME -config" 2>/dev/null; then
      log "已停止残留的 $BINARY_NAME 进程"
      killed=1
    fi
  fi
  return $((1 - killed))
}

# ----------------------------- 启动进程 -------------------------------------
write_systemd_unit() {
  local unit="/etc/systemd/system/$SERVICE_NAME.service"
  log "写入 systemd 服务单元 $unit ..."
  {
    cat <<EOF
[Unit]
Description=$APP_NAME v2 - 业余无线电 QSO/QSL 日志与卡片管理系统
After=network.target

[Service]
Type=simple
User=$SYSTEMD_USER
Group=$SYSTEMD_USER
WorkingDirectory=$DEPLOY_DIR
ExecStart=$BINARY -config $CONFIG_FILE
Restart=always
RestartSec=$RESTART_DELAY
EOF
    [ -n "${LITEQSL_SECRET_KEY:-}" ] && printf 'Environment=SECRET_KEY=%s\n' "$LITEQSL_SECRET_KEY"
    cat <<EOF

NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$DATA_DIR

[Install]
WantedBy=multi-user.target
EOF
  } > "$unit" || die "写入 systemd 单元失败"
  systemctl daemon-reload || die "systemctl daemon-reload 失败"
  systemctl enable "$SERVICE_NAME" -q || die "systemctl enable 失败"
}

start_systemd() {
  local unit="/etc/systemd/system/$SERVICE_NAME.service"
  if [ ! -f "$unit" ] || [ -n "${LITEQSL_REINSTALL_UNIT:-}" ]; then
    write_systemd_unit
  fi
  log "启动 systemd 服务 $SERVICE_NAME ..."
  systemctl restart "$SERVICE_NAME" || die "systemctl restart 失败"
  log "已通过 systemd 启动（Restart=always，开机自启）"
  log "手动停止: systemctl stop $SERVICE_NAME"
}

start_supervised() {
  rm -f "$STOP_FLAG" "$CHILD_PID_FILE"
  cd "$DEPLOY_DIR" || die "无法进入目录 $DEPLOY_DIR"
  log "以守护循环方式启动（进程崩溃后 ${RESTART_DELAY}s 自动重启）..."
  nohup "$SCRIPT_PATH" __supervise >> "$LOG_FILE" 2>&1 &
  echo $! > "$PID_FILE"
  log "已启动，守护进程 pid=$(cat "$PID_FILE")，日志: $LOG_FILE"
  log "手动停止: $0 stop"
}

# 守护循环：反复拉起服务，仅当检测到停止标记时才退出
supervise_loop() {
  cd "$DEPLOY_DIR" || exit 1
  local child="" code=0

  # 收到 TERM/INT 时同步结束子进程，避免残留
  cleanup() {
    if [ -n "$child" ] && kill -0 "$child" 2>/dev/null; then
      kill "$child" 2>/dev/null || true
    fi
    rm -f "$CHILD_PID_FILE"
    exit 0
  }
  trap cleanup TERM INT

  while true; do
    # 后台启动并把子进程 PID 写入文件，便于 stop 精确结束（兼容 Git Bash/MSYS）
    "$BINARY" -config "$CONFIG_FILE" &
    child=$!
    echo "$child" > "$CHILD_PID_FILE"

    wait "$child"
    code=$?
    child=""

    if [ -f "$STOP_FLAG" ]; then
      rm -f "$STOP_FLAG" "$CHILD_PID_FILE"
      log "收到停止标记，守护进程退出"
      exit 0
    fi
    log "$BINARY_NAME 退出 (code=$code)，${RESTART_DELAY}s 后自动重启 ..."
    sleep "$RESTART_DELAY"
  done
}

start_process() {
  ensure_layout
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
      log "健康检查未通过（服务可能尚未就绪，或端口不是 $PORT）"
    fi
  fi
}

usage() {
  cat <<EOF
用法: $0 [命令]

  不带参数 / deploy   一键部署：停止 -> 获取程序 -> 准备布局 -> 启动
  start               停止旧进程并启动
  stop                停止服务
  restart             重启服务
  update              更新程序（下载/构建）并重启
  build               仅用 Go 从源码构建二进制
  status              查看运行状态与健康检查
  install-service     仅安装 systemd 服务

环境变量:
  LITEQSL_RELEASE_URL   发布下载基地址（默认本仓库 Releases 最新版）
  LITEQSL_HOST          监听地址（默认 0.0.0.0）
  LITEQSL_PORT          监听端口（默认 8000）
  LITEQSL_USER          systemd 运行用户（默认当前用户）
  LITEQSL_SECRET_KEY    会话密钥
  LITEQSL_RESTART_DELAY 崩溃后重启延迟秒数（默认 5）
  GO                    go 命令（默认 go）
EOF
}

# ----------------------------- 主流程 ---------------------------------------
main() {
  case "${1:-}" in
    __supervise) supervise_loop ;;
    start)
      stop_process
      fetch_binary 0
      start_process
      ;;
    stop)
      stop_process
      ;;
    restart)
      stop_process
      fetch_binary 0
      start_process
      ;;
    update)
      stop_process
      fetch_binary 1
      start_process
      ;;
    build)
      build_binary
      ;;
    status)
      status
      ;;
    install-service)
      ensure_layout
      has_systemd || die "当前环境不可用 systemd（需 root 且 systemd 正在运行）"
      write_systemd_unit
      log "systemd 服务已安装。启动: systemctl start $SERVICE_NAME"
      ;;
    "" | deploy)
      log "===== 步骤 1/4: 停止现有进程 ====="
      stop_process
      log "===== 步骤 2/4: 准备程序 ====="
      fetch_binary 0
      log "===== 步骤 3/4: 准备运行布局 ====="
      ensure_layout
      log "===== 步骤 4/4: 启动进程 ====="
      start_process
      log "===== 部署完成 ====="
      sleep 1
      status
      ;;
    help | -h | --help)
      usage
      ;;
    *)
      usage
      exit 1
      ;;
  esac
}

main "$@"
