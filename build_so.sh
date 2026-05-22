#!/bin/bash
set -e

APP_NAME="goProxy"
BUILD_DIR="build"
SO_NAME="libgoproxy.so"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

if [ -f "$SCRIPT_DIR/android-ndk-r26b/toolchains/llvm/prebuilt/windows-x86_64/bin/aarch64-linux-android24-clang.cmd" ]; then
    NDK_ROOT="$SCRIPT_DIR/android-ndk-r26b"
elif [ -f "$SCRIPT_DIR/../android-ndk-r26b/toolchains/llvm/prebuilt/windows-x86_64/bin/aarch64-linux-android24-clang.cmd" ]; then
    NDK_ROOT="$SCRIPT_DIR/../android-ndk-r26b"
else
    echo "错误: 未找到 android-ndk-r26b"
    echo "请将 NDK 解压到 $SCRIPT_DIR/ 或上级目录"
    exit 1
fi

echo "NDK 路径: $NDK_ROOT"

# 检测主机平台
if [ -d "$NDK_ROOT/toolchains/llvm/prebuilt/windows-x86_64/bin" ]; then
    HOST_TAG="windows-x86_64"
elif [ -d "$NDK_ROOT/toolchains/llvm/prebuilt/linux-x86_64/bin" ]; then
    HOST_TAG="linux-x86_64"
elif [ -d "$NDK_ROOT/toolchains/llvm/prebuilt/darwin-x86_64/bin" ]; then
    HOST_TAG="darwin-x86_64"
else
    echo "错误: 未找到 NDK 工具链"
    exit 1
fi

NDK_BIN="$NDK_ROOT/toolchains/llvm/prebuilt/$HOST_TAG/bin"
echo "使用工具链: $NDK_BIN"

GOHOSTARCH="$(go env GOHOSTARCH)"
GOVERSION="$(go env GOVERSION)"
echo "GOHOSTARCH: $GOHOSTARCH"
echo "GOVERSION: $GOVERSION"
if [ "$GOHOSTARCH" = "386" ]; then
    echo "错误: 检测到 32 位 Go 工具链。编译 Android arm/arm64 JNI .so 需要 64 位主机 Go。"
    echo "请安装 64 位 Go 后再执行 build_so.sh"
    exit 1
fi

mkdir -p "$BUILD_DIR"

# 编译目标: arm64-v8a 和 armeabi-v7a
declare -A TARGETS
TARGETS["arm64-v8a"]="aarch64-linux-android24"
TARGETS["armeabi-v7a"]="armv7a-linux-androideabi24"

for ABI in "${!TARGETS[@]}"; do
    CLANG_PREFIX="${TARGETS[$ABI]}"
    CC="$NDK_BIN/${CLANG_PREFIX}-clang"

    # Windows 环境尝试添加扩展名
    if [ ! -f "$CC" ]; then
        if [ -f "${CC}.cmd" ]; then
            CC="${CC}.cmd"
        elif [ -f "${CC}.exe" ]; then
            CC="${CC}.exe"
        else
            echo "警告: 编译器不存在 $CC, 跳过 $ABI"
            continue
        fi
    fi

    echo ""
    echo "========================================="
    echo "编译 $SO_NAME - $ABI"
    echo "CC: $CC"
    echo "========================================="

    export CGO_ENABLED=1
    export GOOS=android
    if [ "$ABI" = "arm64-v8a" ]; then
        export GOARCH=arm64
    else
        export GOARCH=arm
        export GOARM=7
    fi
    export CC="$CC"

    OUTPUT_DIR="$BUILD_DIR/$ABI"
    mkdir -p "$OUTPUT_DIR"

    go build -buildmode=c-shared -o "$OUTPUT_DIR/$SO_NAME" \
        -ldflags="-w -s" \
        -trimpath \
        .

    # strip
    STRIP="$NDK_BIN/llvm-strip"
    if [ -f "${STRIP}.exe" ]; then STRIP="${STRIP}.exe"; fi
    if [ -f "$STRIP" ]; then
        "$STRIP" "$OUTPUT_DIR/$SO_NAME"
        echo "strip 完成"
    fi

    # 复制头文件 (Go 自动生成的)
    HEADER_FILE="$OUTPUT_DIR/${SO_NAME%.so}.h"
    if [ -f "$HEADER_FILE" ]; then
        echo "头文件已生成: $HEADER_FILE"
    fi

    # 复制到 jar/assets_so 目录
    JAR_ASSETS="$SCRIPT_DIR/../CatVodSpider/jar/assets_so"
    if [ -d "$JAR_ASSETS" ]; then
        mkdir -p "$JAR_ASSETS/$ABI"
        cp "$OUTPUT_DIR/$SO_NAME" "$JAR_ASSETS/$ABI/"
        echo "已复制到 $JAR_ASSETS/$ABI/$SO_NAME"
    fi

    FILE_SIZE=$(du -h "$OUTPUT_DIR/$SO_NAME" | cut -f1)
    echo "完成: $ABI ($FILE_SIZE)"
done

echo ""
echo "========================================="
echo "全部编译完成! 输出目录: $BUILD_DIR/"
echo "========================================="
ls -la "$BUILD_DIR"/*/  2>/dev/null || true
