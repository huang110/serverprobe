@echo off
setlocal
set ROOT=%~dp0..
cd /d "%ROOT%"
docker compose down
echo ServerProbe 已停止
endlocal
