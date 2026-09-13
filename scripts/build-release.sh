#!/usr/bin/env bash
# =============================================================================
# LiteQSL-Web v2 多平台构建脚本
#
# 用法:
#   ./scripts/build-release.sh            # 版本号自动从 internal/version/version.go 读取
#   ./scripts/build-release.sh 2.0.1      # 指定版本号
#
# 产物:
#   dist/liteqsl-<os>-<arch>[.exe]                    纯二进制
#   dist/liteqsl-<version>-<os>-<arch>.tar.gz         发布包（含二进制/static/配置/部署文件）
#
# 依赖: Go 1.26+（CGO_ENABLED=0，纯 Go SQLite，无需 C 工具链）
# =============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

# 版本号：优先命令行参数，其次从 version.go 提取
if [ $# -ge 1 ]; then
  VERSION="$1"
else
  VERSION="$(grep -oE 'AppVersion = "[^"]+"' internal/version/version.go | head -1 | sed -E 's/.*"([^"]+)".*/\1/')"
fi
[ -n "$VERSION" ] || { echo "无法确定版本号" >&2; exit 1; }

DIST="$ROOT/dist"
rm -rf "$DIST"
mkdir -p "$DIST"

GO="${GO:-go}"
command -v "$GO" >/dev/null 2>&1 || { echo "未找到 go 命令，请安装 Go 1.26+ 或设置 GO 环境变量" >&2; exit 1; }

# 平台矩阵：os/arch/arm-version
PLATFORMS=(
  "linux/amd64/"
  "linux/arm64/"
  "linux/arm/7"
  "windows/amd64/"
  "darwin/amd64/"
  "darwin/arm64/"
)

export CGO_ENABLED=0

log() { printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"; }

for entry in "${PLATFORMS[@]}"; do
  IFS='/' read -r os arch arm <<< "$entry"
  ext=""
  [ "$os" = "windows" ] && ext=".exe"

  out="$DIST/liteqsl-${os}-${arch}${ext}"
  export GOOS="$os" GOARCH="$arch"
  if [ -n "$arm" ]; then
    export GOARM="$arm"
  else
    unset GOARM || true
  fi

  log "构建 ${os}/${arch}${arm:+ (GOARM=$arm)} ..."
  "$GO" build -trimpath -ldflags "-s -w" -o "$out" ./cmd/liteqsl

  # 打包发布包
  pkgdir="$DIST/pkg-${os}-${arch}"
  mkdir -p "$pkgdir"
  cp "$out" "$pkgdir/liteqsl${ext}"
  cp -r "$ROOT/static" "$pkgdir/static"
  cp "$ROOT/config.example.yaml" "$pkgdir/config.example.yaml"
  cp "$ROOT/deploy.sh" "$pkgdir/deploy.sh"
  cp -r "$ROOT/deploy" "$pkgdir/deploy"
  cp "$ROOT/README.md" "$pkgdir/README.md"
  mkdir -p "$pkgdir/docs"
  cp "$ROOT/docs/rebuild_reports/deployment.md" "$pkgdir/docs/" 2>/dev/null || true
  cp "$ROOT/docs/rebuild_reports/api-compatibility.md" "$pkgdir/docs/" 2>/dev/null || true
  cp "$ROOT/docs/v2.0.0-changelog.md" "$pkgdir/docs/" 2>/dev/null || true

  ( cd "$DIST" && tar -czf "liteqsl-${VERSION}-${os}-${arch}.tar.gz" -C "$pkgdir" . )
  rm -rf "$pkgdir"
done

log "构建完成，产物在 $DIST"

# 生成校验和清单（发布时一并上传）
if command -v sha256sum >/dev/null 2>&1; then
  ( cd "$DIST" && sha256sum liteqsl-*.tar.gz > SHA256SUMS )
elif command -v shasum >/dev/null 2>&1; then
  ( cd "$DIST" && shasum -a 256 liteqsl-*.tar.gz > SHA256SUMS )
fi
[ -f "$DIST/SHA256SUMS" ] && log "已生成校验和清单: $DIST/SHA256SUMS"

ls -lh "$DIST" | sed '1d'
