# GoProxyAndroid

面向 Android 场景的 Go 代理服务，主要用于 CatVod 相关项目中的本地代理能力。

这个仓库包含 Go 代理的核心源码与构建脚本，可用于产出：

- Android JNI 动态库：`libgoproxy.so`
- 兼容旧打包流程的 Android 可执行二进制

## 项目结构

- `main.go`：独立运行模式下的 HTTP 入口
- `proxy.go`：代理核心逻辑，负责分块下载、并发拉流、Range 透传与重试
- `proxy_jni.go`：JNI 桥接层，用于把 Go 代理编译成 Android 可加载的 `.so`
- `build_so.bat`：Windows 下编译 Android `.so`
- `build_so.sh`：Bash 环境下编译 Android `.so`
- `build.sh`：旧版独立可执行文件构建脚本，保留作兼容用途

## 使用场景

当前主用途是把 Go 代理编译为：

- `build/arm64-v8a/libgoproxy.so`
- `build/armeabi-v7a/libgoproxy.so`

随后再由上层 Android 项目将这些文件打包进资源目录并在运行时加载。

## 构建要求

- Go 1.22 或更高版本
- Android NDK `r26b`

## 编译 Android `.so`

### Windows

```bat
build_so.bat
```

### Bash / Git Bash / WSL

```bash
chmod +x build_so.sh
./build_so.sh
```

编译完成后，输出文件位于：

```text
build/arm64-v8a/libgoproxy.so
build/armeabi-v7a/libgoproxy.so
```

## 兼容说明

- `build_so.bat` 和 `build_so.sh` 会在检测到同级 `CatVodSpider` 项目时，自动把生成的 `.so` 复制到 `CatVodSpider/jar/assets_so`。
- `build.sh` 仍然保留，用于旧方案下直接构建 Android 可执行代理文件，但当前主链路优先使用 JNI `.so` 方案。

## 代理能力说明

这个代理主要提供以下能力：

- 本地 HTTP 服务入口
- `Range` 请求解析与透传
- 基于分块的并发下载
- 下载失败自动重试
- `/health` 健康检查接口

## 鸣谢

本项目基于不夜 `@sifanss` 分享的代理源码进行二次开发与整理。

在原有实现基础上，我补充和调整了 Android JNI `.so` 构建链路、仓库结构、文档说明以及与上层项目的集成方式。

## 开源说明

这个仓库只保留真正参与构建和发布的核心源码与脚本，不包含历史备份、实验目录、NDK 压缩包或上层业务项目文件。

## License

MIT
