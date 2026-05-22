#!/bin/bash

appName="goProxy"

BuildReleaseAndroid() {
  mkdir -p "build"
  
  # Windows 系统可能需要手动下载 NDK，这里提供下载和解压命令
  # 注意：Windows 下可能需要安装 unzip 工具
  if [ ! -d "android-ndk-r26b" ]; then
    echo "下载 Android NDK..."
    # Windows 下使用 curl 或 PowerShell 下载
    if command -v curl &> /dev/null; then
      curl -L -o android-ndk-r26b-windows.zip https://dl.google.com/android/repository/android-ndk-r26b-windows.zip
    else
      # 如果 curl 不可用，提示手动下载
      echo "请手动下载 NDK: https://dl.google.com/android/repository/android-ndk-r26b-windows.zip"
      echo "并解压到当前目录，然后重新运行脚本"
      return 1
    fi
    
    # 解压 NDK
    if command -v unzip &> /dev/null; then
      unzip android-ndk-r26b-windows.zip
      # Windows NDK 解压后目录名可能不同
      if [ -d "android-ndk-r26b" ]; then
        echo "NDK 解压完成"
      else
        # 尝试查找解压后的目录
        for dir in android-ndk-*; do
          if [ -d "$dir" ]; then
            mv "$dir" android-ndk-r26b
            echo "重命名 NDK 目录为 android-ndk-r26b"
          fi
        done
      fi
      rm -f android-ndk-r26b-windows.zip
    else
      echo "请安装 unzip 工具或手动解压 NDK"
      return 1
    fi
  fi

  # Windows 下需要根据不同的 Shell 环境调整路径
  OS_ARCHES=("amd64" "arm64" "386" "arm")
  
  # 检测操作系统环境并设置正确的 NDK 路径
  NDK_DIR="android-ndk-r26b"
  
  # 尝试不同的 NDK 可执行文件路径
  if [ -d "$NDK_DIR/toolchains/llvm/prebuilt/windows-x86_64/bin" ]; then
    # Windows 原生路径
    NDK_BIN_PATH="$NDK_DIR/toolchains/llvm/prebuilt/windows-x86_64/bin"
    CGO_ARGS=("x86_64-linux-android24-clang.cmd" "aarch64-linux-android24-clang.cmd" "i686-linux-android24-clang.cmd" "armv7a-linux-androideabi24-clang.cmd")
  elif [ -d "$NDK_DIR/toolchains/llvm/prebuilt/windows" ]; then
    # 另一种可能的 Windows 路径
    NDK_BIN_PATH="$NDK_DIR/toolchains/llvm/prebuilt/windows/bin"
    CGO_ARGS=("x86_64-linux-android24-clang.exe" "aarch64-linux-android24-clang.exe" "i686-linux-android24-clang.exe" "armv7a-linux-androideabi24-clang.exe")
  elif [ -d "$NDK_DIR/toolchains/llvm/prebuilt/linux-x86_64/bin" ]; then
    # WSL 环境（Linux 路径）
    NDK_BIN_PATH="$NDK_DIR/toolchains/llvm/prebuilt/linux-x86_64/bin"
    CGO_ARGS=("x86_64-linux-android24-clang" "aarch64-linux-android24-clang" "i686-linux-android24-clang" "armv7a-linux-androideabi24-clang")
  else
    echo "未找到 NDK 工具链，请检查 NDK 安装"
    return 1
  fi
  
  echo "使用 NDK 路径: $NDK_BIN_PATH"
  
  for i in "${!OS_ARCHES[@]}"; do
    os_arch=${OS_ARCHES[$i]}
    cgo_cc="$NDK_BIN_PATH/${CGO_ARGS[$i]}"
    
    # 检查编译器是否存在
    if [ ! -f "$cgo_cc" ]; then
      echo "编译器不存在: $cgo_cc"
      echo "尝试查找替代的编译器文件..."
      
      # 尝试查找不带扩展名的版本（WSL/linux环境）
      if [ -f "${cgo_cc%.*}" ]; then
        cgo_cc="${cgo_cc%.*}"
      elif [ -f "${cgo_cc%.cmd}" ]; then
        cgo_cc="${cgo_cc%.cmd}"
      elif [ -f "${cgo_cc%.exe}" ]; then
        cgo_cc="${cgo_cc%.exe}"
      else
        echo "找不到编译器，跳过架构 $os_arch"
        continue
      fi
    fi
    
    echo "为 android-$os_arch 构建..."
    
    # 设置 Go 交叉编译环境变量
    export GOOS=android
    export GOARCH=${os_arch}
    export CC="$cgo_cc"
    export CGO_ENABLED=1
    
    # 如果是 386 架构，Go 中对应的是 386
    if [ "$os_arch" = "386" ]; then
      export GOARCH=386
    fi
    
    # 编译命令
    go build -o "./build/$appName-android-$os_arch" \
      -ldflags="-w -s" \
      -tags=jsoniter .
    
    # 如果存在 strip 工具，执行 strip
    if [ -f "$NDK_BIN_PATH/llvm-strip" ] || [ -f "$NDK_BIN_PATH/llvm-strip.exe" ]; then
      # 根据实际文件扩展名决定
      if [ -f "$NDK_BIN_PATH/llvm-strip.exe" ]; then
        "$NDK_BIN_PATH/llvm-strip.exe" "./build/$appName-android-$os_arch"
      elif [ -f "$NDK_BIN_PATH/llvm-strip.cmd" ]; then
        "$NDK_BIN_PATH/llvm-strip.cmd" "./build/$appName-android-$os_arch"
      else
        "$NDK_BIN_PATH/llvm-strip" "./build/$appName-android-$os_arch"
      fi
    else
      echo "llvm-strip 不可用，跳过 strip 步骤"
    fi
    
    # 如果 upx 可用，执行压缩
    if command -v upx &> /dev/null; then
      upx "./build/$appName-android-$os_arch"
    else
      echo "upx 不可用，跳过压缩步骤"
      echo "可以从 https://upx.github.io/ 下载 upx"
    fi
    
    echo "完成构建: $appName-android-$os_arch"
  done
  
  echo "所有 Android 版本构建完成！输出文件在 build/ 目录"
}

# PowerShell 兼容版本（如果需要）
BuildReleaseAndroidPowerShell() {
  # 这是 PowerShell 版本的函数
  # 保存为 .ps1 文件运行
  Write-Host "PowerShell 版本的构建脚本..."
  Write-Host "建议使用 Git Bash 或 WSL 运行 Bash 版本"
}

# 检测运行环境并提示
if [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "win32" ]]; then
  echo "Windows 环境检测到"
  echo "建议使用 Git Bash 或 WSL 运行此脚本"
  
  # 检查是否在 Git Bash 中
  if [ -n "$MSYSTEM" ]; then
    echo "在 Git Bash 环境中，继续执行..."
  else
    echo "警告：当前可能不是 Bash 环境"
    echo "是否继续？(y/n)"
    read -r answer
    if [ "$answer" != "y" ] && [ "$answer" != "Y" ]; then
      exit 1
    fi
  fi
fi

# 运行构建函数
BuildReleaseAndroid