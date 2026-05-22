# GoProxyAndroid

Android-oriented Go proxy service used by CatVod-based projects.

This project contains the Go sources and build scripts used to produce:

- Android JNI shared libraries: `libgoproxy.so`
- Optional Android executable binaries for legacy packaging paths

## Files

- `main.go`: standalone HTTP proxy entrypoint
- `proxy.go`: core ranged-download proxy implementation
- `proxy_jni.go`: JNI bridge used for `c-shared` Android builds
- `build_so.bat`: build `.so` files on Windows
- `build_so.sh`: build `.so` files on Bash environments
- `build.sh`: legacy script for standalone Android executable builds

## Build `.so` for Android

Requirements:

- Go 1.22+
- Android NDK `r26b`

Windows:

```bat
build_so.bat
```

Bash:

```bash
chmod +x build_so.sh
./build_so.sh
```

Build output:

- `build/arm64-v8a/libgoproxy.so`
- `build/armeabi-v7a/libgoproxy.so`

## Notes

- `build_so.*` will also copy generated `.so` files into a sibling `CatVodSpider/jar/assets_so` directory if it exists.
- The standalone binary build path is kept for compatibility, but the current primary integration path is the JNI `.so` build.

## License

MIT
