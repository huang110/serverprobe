@echo off
setlocal

set ROOT=%~dp0..
cd /d "%ROOT%"

if "%GITHUB_USER%"=="" set GITHUB_USER=huang110
if "%REPO_NAME%"=="" set REPO_NAME=serverprobe

echo === ServerProbe GitHub 发布准备 ===

for %%F in (.gitignore LICENSE README.md README_zh.md CHANGELOG.md CONTRIBUTING.md CODE_OF_CONDUCT.md .github\workflows\ci.yml .github\workflows\docker.yml .github\workflows\release.yml) do (
    if not exist "%%F" (
        echo 缺少文件：%%F
        exit /b 1
    )
)

echo 清理临时文件...
if exist frontend\dist rmdir /s /q frontend\dist
if exist frontend\.vite rmdir /s /q frontend\.vite

echo 初始化 Git...
if not exist .git git init

git add .
git commit -m "feat: initial commit - ServerProbe v1.0.0"
git branch develop 2>nul

echo.
echo 下一步：
echo 1. 在 https://github.com/new 创建仓库 %REPO_NAME%
echo 2. 执行：
echo    git remote add origin https://github.com/%GITHUB_USER%/%REPO_NAME%.git
echo    git branch -M main
echo    git push -u origin main
echo    git push -u origin develop
echo    git tag -a v1.0.0 -m "Release v1.0.0"
echo    git push origin v1.0.0

endlocal
