@echo off
setlocal enabledelayedexpansion

if "%VERSION%"=="" set VERSION=1.0.0
set BUILD_DIR=dist
set APP_NAME=qtrp

echo ========================================
echo QTRP Build Script
echo Version: %VERSION%
echo ========================================
echo.

if exist %BUILD_DIR% (
    echo Cleaning build directory...
    rmdir /s /q %BUILD_DIR%
)
mkdir %BUILD_DIR%

echo.
echo Building Windows AMD64...
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -ldflags="-s -w" -o "%BUILD_DIR%\%APP_NAME%-server-windows-amd64.exe" ./cmd/server
if errorlevel 1 goto error
go build -ldflags="-s -w" -o "%BUILD_DIR%\%APP_NAME%-client-windows-amd64.exe" ./cmd/client
if errorlevel 1 goto error
echo Done: %BUILD_DIR%\%APP_NAME%-*-windows-amd64.exe

echo.
echo Building Linux AMD64...
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-s -w" -o "%BUILD_DIR%\%APP_NAME%-server-linux-amd64" ./cmd/server
if errorlevel 1 goto error
go build -ldflags="-s -w" -o "%BUILD_DIR%\%APP_NAME%-client-linux-amd64" ./cmd/client
if errorlevel 1 goto error
echo Done: %BUILD_DIR%\%APP_NAME%-*-linux-amd64

echo.
echo Building Linux ARM64...
set GOOS=linux
set GOARCH=arm64
go build -ldflags="-s -w" -o "%BUILD_DIR%\%APP_NAME%-server-linux-arm64" ./cmd/server
if errorlevel 1 goto error
go build -ldflags="-s -w" -o "%BUILD_DIR%\%APP_NAME%-client-linux-arm64" ./cmd/client
if errorlevel 1 goto error
echo Done: %BUILD_DIR%\%APP_NAME%-*-linux-arm64

if exist configs (
    echo.
    echo Copying config files...
    xcopy /s /i /q configs "%BUILD_DIR%\configs" >nul
)

echo.
echo ========================================
echo Build complete!
echo ========================================
echo.
dir /b %BUILD_DIR%
echo.
goto end

:error
echo.
echo ========================================
echo Build failed!
echo ========================================
exit /b 1

:end
endlocal
