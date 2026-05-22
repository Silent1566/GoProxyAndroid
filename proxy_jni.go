package main

/*
#include <stdlib.h>
#include <jni.h>

static jstring NewJString(JNIEnv* env, const char* s) {
    if (s == NULL) s = "";
    return (*env)->NewStringUTF(env, s);
}
*/
import "C"

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
	"unsafe"
)

var (
	server        *http.Server
	serverMu      sync.Mutex
	serverRunning bool
	serverPort    int
	lastError     string
	routesOnce    sync.Once
)

func setLastError(msg string) {
	serverMu.Lock()
	lastError = msg
	serverMu.Unlock()
}

func clearLastError() {
	setLastError("")
}

// Java_com_github_catvod_spider_GoProxyLibrary_startProxy
// 供 Android 侧通过 JNI 调用，用于启动 Go 代理服务。
//export Java_com_github_catvod_spider_GoProxyLibrary_startProxy
func Java_com_github_catvod_spider_GoProxyLibrary_startProxy(env *C.JNIEnv, clazz C.jclass, cPort C.jint) C.jint {
	port := int(cPort)
	if port <= 0 {
		port = 5576
	}
	clearLastError()

	serverMu.Lock()
	if serverRunning {
		serverMu.Unlock()
		// 已经处于运行状态时返回 1，避免重复启动。
		setLastError("proxy already running")
		return C.jint(1)
	}
	serverMu.Unlock()

	// 只注册一次 HTTP 路由，避免重复 start 时向默认 ServeMux 重复注册导致 panic。
	routesOnce.Do(setupRoutes)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		log.Printf("Go代理端口绑定失败: %v", err)
		setLastError(err.Error())
		return C.jint(2)
	}

	serverMu.Lock()
	server = srv
	serverRunning = true
	serverPort = port
	serverMu.Unlock()

	go func(localServer *http.Server, listener net.Listener, localPort int) {
		log.SetOutput(os.Stdout)
		log.Printf("Go代理服务启动在 :%d", localPort)
		if err := localServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("Go代理服务错误: %v", err)
			setLastError(err.Error())
		}
		serverMu.Lock()
		if server == localServer {
			serverRunning = false
			server = nil
			serverPort = 0
		}
		serverMu.Unlock()
	}(srv, ln, port)

	return C.jint(0)
}

// Java_com_github_catvod_spider_GoProxyLibrary_stopProxy
// 供 Android 侧停止当前运行中的 Go 代理服务。
//export Java_com_github_catvod_spider_GoProxyLibrary_stopProxy
func Java_com_github_catvod_spider_GoProxyLibrary_stopProxy(env *C.JNIEnv, clazz C.jclass) C.jint {
	serverMu.Lock()
	localServer := server
	running := serverRunning
	serverMu.Unlock()

	if !running || localServer == nil {
		clearLastError()
		return C.jint(0)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := localServer.Shutdown(ctx); err != nil {
		log.Printf("Go代理停止错误: %v", err)
		setLastError(err.Error())
		return C.jint(1)
	}

	serverMu.Lock()
	if server == localServer {
		serverRunning = false
		server = nil
		serverPort = 0
	}
	serverMu.Unlock()
	log.Printf("Go代理服务已停止")
	clearLastError()
	return C.jint(0)
}

// Java_com_github_catvod_spider_GoProxyLibrary_isProxyRunning
// 返回当前 Go 代理在 JNI 模式下是否仍被标记为运行中。
//export Java_com_github_catvod_spider_GoProxyLibrary_isProxyRunning
func Java_com_github_catvod_spider_GoProxyLibrary_isProxyRunning(env *C.JNIEnv, clazz C.jclass) C.jint {
	serverMu.Lock()
	defer serverMu.Unlock()
	if serverRunning {
		return C.jint(1)
	}
	return C.jint(0)
}

// Java_com_github_catvod_spider_GoProxyLibrary_getLastError
//export Java_com_github_catvod_spider_GoProxyLibrary_getLastError
func Java_com_github_catvod_spider_GoProxyLibrary_getLastError(env *C.JNIEnv, clazz C.jclass) C.jstring {
	serverMu.Lock()
	msg := lastError
	serverMu.Unlock()
	cmsg := C.CString(msg)
	defer C.free(unsafe.Pointer(cmsg))
	return C.NewJString(env, cmsg)
}

// setupRoutes 为 JNI 运行模式注册 HTTP 路由。
// 它和独立运行模式中的接口保持一致，方便上层统一接入。
func setupRoutes() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	})

	http.HandleFunc("/proxy", func(w http.ResponseWriter, r *http.Request) {
		params := r.URL.Query()
		thread, chunkSize, url := params.Get("thread"), params.Get("chunkSize"), params.Get("url")

		if thread == "" || chunkSize == "" || url == "" {
			http.Error(w, "参数不完整", http.StatusBadRequest)
			return
		}

		t, err := strconv.Atoi(thread)
		if err != nil {
			http.Error(w, "thread必须为整数", http.StatusBadRequest)
			return
		}
		c, err := strconv.Atoi(chunkSize)
		if err != nil {
			http.Error(w, "chunkSize必须为整数", http.StatusBadRequest)
			return
		}

		player := NewPlayer(r.Header, t, c, url)
		if err := player.Play(w, r.Context()); err != nil {
			// 已经进入流式输出阶段时，只记录日志，不额外改写响应体。
			log.Printf("播放错误: %v", err)
		}
	})

	// 供 Java 层快速探活，避免仅凭端口占用判断代理状态。
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		serverMu.Lock()
		listenPort := serverPort
		serverMu.Unlock()
		if listenPort <= 0 {
			listenPort = 5575
		}
		fmt.Fprintf(w, `{"status": "healthy", "type": "go", "port": %d, "timestamp": "%s"}`, listenPort, time.Now().Format(time.RFC3339))
	})
}

func init() {}

