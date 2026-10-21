@echo off
setlocal

set ROOT=%~dp0..
cd /d "%ROOT%"

echo === ServerProbe 测试环境检查 ===

where go >nul 2>nul
if errorlevel 1 (
    echo 缺少依赖：go
    exit /b 1
)
echo OK：go

where node >nul 2>nul
if errorlevel 1 (
    echo 缺少依赖：node
    exit /b 1
)
echo OK：node

where npm >nul 2>nul
if errorlevel 1 (
    echo 缺少依赖：npm
    exit /b 1
)
echo OK：npm

where docker >nul 2>nul
if errorlevel 1 (
    echo 提示：未安装 Docker，将跳过后端容器测试
) else (
    echo OK：docker
)

if not exist server\config.yaml (
    copy server\config.example.yaml server\config.yaml >nul
    echo 已生成 server\config.yaml
)

if not exist agent\config.yaml (
    copy agent\config.example.yaml agent\config.yaml >nul
    echo 已生成 agent\config.yaml
)

if not exist .env (
    copy .env.example .env >nul
    echo 已生成 .env
)

echo 检查 Go 依赖...
pushd server
go mod download
popd
pushd agent
go mod download
popd
pushd tests
go mod download
popd

echo.
echo 测试环境准备完成。
echo 运行集成测试：scripts\run-integration-tests.bat
endlocal
