#!/usr/bin/env bash
# EOS CLI 官方安装脚本（macOS / Linux）
#
# 用法：
#   curl -fsSL https://raw.githubusercontent.com/dreamSailing/eos/main/scripts/install.sh | bash
#   ./install.sh --version v1.0.0-beta.3     # 安装指定版本
#   ./install.sh --bin ~/.local/bin --dir ~/.local/share/eos
#
# 行为：GitHub Releases 拉取对应平台归档 → SHA256 校验 → 安装到
#   ~/.local/share/eos（eos + core/ 整树）→ 符号链接 ~/.local/bin/eos。
# `eos update` 之后升级同一位置，符号链接保持不变。

set -euo pipefail

REPO="dreamSailing/eos"
VERSION=""
BIN_DIR="${EOS_INSTALL_BIN:-$HOME/.local/bin}"
DIST_DIR="${EOS_INSTALL_DIR:-$HOME/.local/share/eos}"

while [ $# -gt 0 ]; do
  case "$1" in
    --version) VERSION="${2:-}"; shift 2 ;;
    --bin) BIN_DIR="${2:-}"; shift 2 ;;
    --dir) DIST_DIR="${2:-}"; shift 2 ;;
    -h|--help)
      cat <<'USAGE'
用法: install.sh [--version vX.Y.Z] [--bin DIR] [--dir DIR]
  --version  安装指定版本（默认最新）
  --bin      可执行文件链接目录（默认 ~/.local/bin，可用 EOS_INSTALL_BIN 覆盖）
  --dir      安装目录（默认 ~/.local/share/eos，可用 EOS_INSTALL_DIR 覆盖）
USAGE
      exit 0 ;;
    *) echo "未知参数: $1（--help 查看用法）" >&2; exit 1 ;;
  esac
done

err() { echo "错误: $*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || err "缺少依赖 $1，请先安装"; }
need curl; need tar

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  darwin|linux) ;;
  *) err "不支持的系统: $(uname -s)（本脚本支持 macOS / Linux）" ;;
esac

ARCH="$(uname -m)"
case "$ARCH" in
  arm64|aarch64) ARCH="arm64" ;;
  x86_64|amd64)  ARCH="amd64" ;;
  *) err "不支持的架构: $(uname -m)" ;;
esac

SHA_CMD="sha256sum"; command -v sha256sum >/dev/null 2>&1 || SHA_CMD="shasum -a 256"

# 解析最新版本（未指定 --version 时）
if [ -z "$VERSION" ]; then
  echo "正在获取最新版本..."
  VERSION="$(curl -fsSL --retry 3 \
    -H "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/${REPO}/releases/latest" \
    | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)"
  [ -n "$VERSION" ] || err "无法获取最新版本号，请到 https://github.com/${REPO}/releases 手动下载"
fi
echo "目标版本: ${VERSION} (${OS}/${ARCH})"

VER_NUM="${VERSION#v}"
ASSET="eos-cli_v${VER_NUM}_${OS}-${ARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "下载 ${ASSET}..."
curl -fsSL --retry 3 -o "${TMP}/${ASSET}" "${BASE_URL}/${ASSET}" \
  || err "下载失败（版本 ${VERSION} 可能没有 ${OS}-${ARCH} 归档），到 ${BASE_URL} 确认"

echo "校验 SHA256..."
curl -fsSL --retry 3 -o "${TMP}/SHA256SUMS.txt" "${BASE_URL}/SHA256SUMS.txt" || err "下载校验清单失败"
WANT="$(awk -v f="$ASSET" '$2 == f {print tolower($1)}' "${TMP}/SHA256SUMS.txt")"
[ -n "$WANT" ] || err "SHA256SUMS.txt 中没有 ${ASSET} 条目"
GOT="$($SHA_CMD "${TMP}/${ASSET}" | awk '{print tolower($1)}')"
[ "$GOT" = "$WANT" ] || err "校验不匹配：期望 ${WANT}，实际 ${GOT}"

echo "安装到 ${DIST_DIR} ..."
mkdir -p "$DIST_DIR" "$BIN_DIR"
# 旧版本先移走（运行中的进程不受影响），失败则清理后重试
OLD=""
if [ -d "${DIST_DIR}/core" ] || [ -e "${DIST_DIR}/eos" ]; then
  OLD="${DIST_DIR}.old-$$"
  mv "$DIST_DIR" "$OLD" 2>/dev/null || { rm -rf "$DIST_DIR"; }
  mkdir -p "$DIST_DIR"
fi
tar -xzf "${TMP}/${ASSET}" -C "$TMP"
SRC_DIR="$(dirname "$(find "${TMP}" -maxdepth 2 -name eos -type f | head -1)")"
cp -R "${SRC_DIR}/." "$DIST_DIR/"
[ -n "$OLD" ] && rm -rf "$OLD" 2>/dev/null || true
chmod +x "${DIST_DIR}/eos"

ln -sf "${DIST_DIR}/eos" "${BIN_DIR}/eos"

case ":${PATH}:" in
  *":${BIN_DIR}:"*) ;;
  *)
    cat <<EOF

注意: ${BIN_DIR} 不在 PATH 中，请把下面这行加入你的 shell 配置（~/.bashrc / ~/.zshrc）：
  export PATH="${BIN_DIR}:\$PATH"
EOF
    ;;
esac

echo
"${BIN_DIR}/eos" --help >/dev/null 2>&1 || true
echo "安装完成: $(${BIN_DIR}/eos version 2>/dev/null || echo "${BIN_DIR}/eos")"
echo "运行 eos 开始使用；eos update 可自升级到最新版。"
