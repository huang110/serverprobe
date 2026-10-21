@echo off
setlocal

set ROOT=%~dp0..
cd /d "%ROOT%"

echo === ServerProbe 集成测试 ===

echo [1/4] Server 测试
pushd server
go test -count=1 ./...
if errorlevel 1 exit /b 1
popd

echo [2/4] 端到端集成测试
pushd tests
go test -v -count=1 -timeout 10m ./...
if errorlevel 1 exit /b 1
popd

echo [3/4] Agent 测试
pushd agent
go test -count=1 ./...
if errorlevel 1 exit /b 1
popd

echo [4/4] 前端构建检查
if exist frontend\node_modules (
    pushd frontend
    call npm run build
    if errorlevel 1 exit /b 1
    popd
) else (
    echo 跳过前端构建：frontend\node_modules 不存在
)

echo.
echo 集成测试完成
endlocal
