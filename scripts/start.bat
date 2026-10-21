@echo off
setlocal
set ROOT=%~dp0..
cd /d "%ROOT%"

where docker >nul 2>nul
if errorlevel 1 (
    echo 错误：未安装 Docker
    exit /b 1
)

echo === ServerProbe 一键启动 ===
if not exist .env copy .env.example .env >nul
if not exist server\config.yaml copy server\config.example.yaml server\config.yaml >nul
if not exist agent\config.yaml copy agent\config.docker.yaml agent\config.yaml >nul

docker compose build
if errorlevel 1 exit /b 1
docker compose up -d
if errorlevel 1 exit /b 1

echo.
echo 前端：http://localhost
echo API：http://localhost:8080
echo 默认登录：admin / admin123
endlocal
