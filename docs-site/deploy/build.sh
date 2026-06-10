#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

if ! command -v npm >/dev/null 2>&1; then
  echo "npm 未安装，请先安装 Node.js 18+" >&2
  exit 1
fi

if [ ! -d node_modules ]; then
  npm install
fi

npm run build

echo "文档站构建完成: $ROOT_DIR/dist"
