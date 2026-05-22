@echo off
setlocal enabledelayedexpansion
chcp 65001 >nul 2>&1

set "SCRIPT_DIR=%~dp0"
set "BUILD_DIR=%SCRIPT_DIR%build"
set "SO_NAME=libgoproxy.so"
set "NDK_ROOT=%SCRIPT_DIR%android-ndk-r26b"

if not exist "%NDK_ROOT%\toolchains\llvm\prebuilt\windows-x86_64\bin\aarch64-linux-android24-clang.cmd" (
    set "NDK_ROOT=%SCRIPT_DIR%..\android-ndk-r26b"
)
if not exist "%NDK_ROOT%\toolchains\llvm\prebuilt\windows-x86_64\bin\aarch64-linux-android24-clang.cmd" (
    echo ERROR: NDK not found
    echo Extract android-ndk-r26b to %SCRIPT_DIR%
    exit /b 1
)

set "NDK_BIN=%NDK_ROOT%\toolchains\llvm\prebuilt\windows-x86_64\bin"
echo NDK: %NDK_BIN%

set "GO_EXE=C:\Program Files (x86)\Go\bin\go.exe"
if not exist "%GO_EXE%" (
    set "GO_EXE=go"
)
echo GO: %GO_EXE%

if not exist "%BUILD_DIR%" mkdir "%BUILD_DIR%"

echo.
echo === Building arm64-v8a ===

set "CGO_ENABLED=1"
set "GOOS=android"
set "GOARCH=arm64"
set "CC=%NDK_BIN%\aarch64-linux-android24-clang.cmd"

if not exist "%BUILD_DIR%\arm64-v8a" mkdir "%BUILD_DIR%\arm64-v8a"

"%GO_EXE%" build -buildmode=c-shared -o "%BUILD_DIR%\arm64-v8a\%SO_NAME%" -ldflags="-w -s" -trimpath .
if errorlevel 1 (
    echo FAILED: arm64-v8a
    exit /b 1
)

if exist "%NDK_BIN%\llvm-strip.exe" (
    "%NDK_BIN%\llvm-strip.exe" "%BUILD_DIR%\arm64-v8a\%SO_NAME%"
    echo Stripped arm64-v8a
)

echo.
echo === Building armeabi-v7a ===

set "GOARCH=arm"
set "GOARM=7"
set "CC=%NDK_BIN%\armv7a-linux-androideabi24-clang.cmd"

if not exist "%BUILD_DIR%\armeabi-v7a" mkdir "%BUILD_DIR%\armeabi-v7a"

"%GO_EXE%" build -buildmode=c-shared -o "%BUILD_DIR%\armeabi-v7a\%SO_NAME%" -ldflags="-w -s" -trimpath .
if errorlevel 1 (
    echo FAILED: armeabi-v7a
    exit /b 1
)

if exist "%NDK_BIN%\llvm-strip.exe" (
    "%NDK_BIN%\llvm-strip.exe" "%BUILD_DIR%\armeabi-v7a\%SO_NAME%"
    echo Stripped armeabi-v7a
)

echo.
echo === Copying to jar/assets ===

set "JAR_ASSETS=%SCRIPT_DIR%..\CatVodSpider\jar\assets_so"
if exist "%JAR_ASSETS%" (
    if not exist "%JAR_ASSETS%\arm64-v8a" mkdir "%JAR_ASSETS%\arm64-v8a"
    copy /Y "%BUILD_DIR%\arm64-v8a\%SO_NAME%" "%JAR_ASSETS%\arm64-v8a\" >nul
    if not exist "%JAR_ASSETS%\armeabi-v7a" mkdir "%JAR_ASSETS%\armeabi-v7a"
    copy /Y "%BUILD_DIR%\armeabi-v7a\%SO_NAME%" "%JAR_ASSETS%\armeabi-v7a\" >nul
    echo Copied to jar/assets_so
)

echo.
echo === Done ===
