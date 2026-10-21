#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GITHUB_USER="${GITHUB_USER:-huang110}"
REPO_NAME="${REPO_NAME:-serverprobe}"

echo "=== ServerProbe GitHub 发布准备 ==="

required=(
    ".gitignore"
    "LICENSE"
    "README.md"
    "README_zh.md"
    "CHANGELOG.md"
    "CONTRIBUTING.md"
    "CODE_OF_CONDUCT.md"
    ".github/workflows/ci.yml"
    ".github/workflows/docker.yml"
    ".github/workflows/release.yml"
    ".github/ISSUE_TEMPLATE/bug_report.yml"
    ".github/ISSUE_TEMPLATE/feature_request.yml"
    ".github/pull_request_template.md"
)
missing=()
for file in "${required[@]}"; do
    [ -f "$file" ] || missing+=("$file")
done
if [ "${#missing[@]}" -gt 0 ]; then
    echo "缺少以下文件："
    printf '  - %s\n' "${missing[@]}"
    exit 1
fi
echo "必需文件检查通过"

echo "清理临时文件..."
find . -name "*.log" -type f -delete 2>/dev/null || true
find . -name "*.tmp" -type f -delete 2>/dev/null || true
find . -name ".DS_Store" -type f -delete 2>/dev/null || true
rm -rf frontend/dist frontend/.vite tests/*.db tests/*.log 2>/dev/null || true

echo "检查敏感信息..."
if grep -r -n -E "(password|secret|token)[[:space:]]*[:=][[:space:]]*[\"'][^\"']{8,}[\"']" \
    server/config.example.yaml server/config.production.yaml agent/config.example.yaml .env.example 2>/dev/null \
    | grep -v -E "change|example|your-|CHANGE|admin123"; then
    echo "发现疑似真实密钥，请检查后再发布"
    exit 1
fi
echo "示例配置检查通过"

if [ ! -d .git ]; then
    git init
    echo "Git 仓库已初始化"
fi

git add .
git commit -m "feat: initial commit - ServerProbe v1.0.0" || echo "没有新的更改需要提交"
git branch develop 2>/dev/null || true

echo ""
echo "下一步："
echo "1. 在 https://github.com/new 创建仓库 ${REPO_NAME}"
echo "2. 执行："
echo "   git remote add origin https://github.com/${GITHUB_USER}/${REPO_NAME}.git"
echo "   git branch -M main"
echo "   git push -u origin main"
echo "   git push -u origin develop"
echo "   git tag -a v1.0.0 -m 'Release v1.0.0'"
echo "   git push origin v1.0.0"
