@echo off
setlocal
echo ========================================================
echo Building CLIProxyAPI Apply Patch Plugin (apply_patch.dll)
echo ========================================================

cd /d "%~dp0"
set CGO_ENABLED=1

echo Running go mod tidy...
go mod tidy
if %ERRORLEVEL% neq 0 (
    echo [ERROR] go mod tidy failed!
    pause
    exit /b %ERRORLEVEL%
)

echo Compiling apply_patch.dll...
go build -buildmode=c-shared -ldflags="-s -w" -o apply_patch.dll .
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Build failed!
    pause
    exit /b %ERRORLEVEL%
)

if exist apply_patch.h del /f /q apply_patch.h

echo.
echo [SUCCESS] apply_patch.dll built successfully!
echo.
pause
