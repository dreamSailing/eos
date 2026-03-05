@echo off
REM embed_gopls.bat - Download and embed gopls binary for LSP support (Windows)
REM Usage: scripts\embed_gopls.bat

setlocal

set GOPSLS_VERSION=v0.21.1
set BIN_DIR=internal\lsp\binaries

if not exist "%BIN_DIR%" mkdir "%BIN_DIR%"

echo Installing gopls %GOPSLS_VERSION%...
go install golang.org/x/tools/gopls@%GOPSLS_VERSION%

REM Get GOPATH
for /f "delims=" %%i in ('go env GOPATH') do set GOPATH=%%i
if "%GOPATH%"=="" set GOPATH=%USERPROFILE%\go

REM Detect current platform
for /f "delims=" %%i in ('go env GOOS') do set GOOS=%%i
for /f "delims=" %%i in ('go env GOARCH') do set GOARCH=%%i

set BINARY_NAME=gopls-%GOOS%-%GOARCH%.exe
set SOURCE=%GOPATH%\bin\gopls.exe
set DEST=%BIN_DIR%\%BINARY_NAME%

echo Copying gopls to %DEST%...
copy "%SOURCE%" "%DEST%"

echo Embedded gopls binary created: %DEST%
echo.
echo To build with embedded gopls:
echo   go build -tags with_gopls

endlocal
