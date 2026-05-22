package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

var (
	server        *http.Server
	serverMu      sync.Mutex
	serverRunning bool
)

// Java_com_github_catvod_spider_GoProxyLibrary_startProxy
// 供 Android 侧通过 JNI 调用，用于启动 Go 代理服务。
//export Java_com_github_catvod_spider_GoProxyLibrary_startProxy
func Java_com_github_catvod_spider_GoProxyLibrary_startProxy(cPort C.int) C.int {
	port := int(cPort)
	if port <= 0 {
		port = 5575
	}

	serverMu.Lock()
	if serverRunning {
		serverMu.Unlock()
		// 已经处于运行状态时返回 1，避免重复启动。
		return C.int(1)
	}
	serverMu.Unlock()

	// 注册一次 HTTP 路由，供 JNI 版本的本地服务使用。
	setupRoutes()

	server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.SetOutput(os.Stdout)
		log.Printf("Go代理服务启动在 :%d", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Go代理服务错误: %v", err)
		}
	}()

	serverMu.Lock()
	serverRunning = true
	serverMu.Unlock()

	return C.int(0)
}

// Java_com_github_catvod_spider_GoProxyLibrary_stopProxy
// 供 Android 侧停止当前运行中的 Go 代理服务。
//export Java_com_github_catvod_spider_GoProxyLibrary_stopProxy
func Java_com_github_catvod_spider_GoProxyLibrary_stopProxy() C.int {
	serverMu.Lock()
	defer serverMu.Unlock()

	if !serverRunning || server == nil {
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Go代理停止错误: %v", err)
		return 1
	}

	serverRunning = false
	server = nil
	log.Printf("Go代理服务已停止")
	return 0
}

// Java_com_github_catvod_spider_GoProxyLibrary_isProxyRunning
// 返回当前 Go 代理在 JNI 模式下是否仍被标记为运行中。
//export Java_com_github_catvod_spider_GoProxyLibrary_isProxyRunning
func Java_com_github_catvod_spider_GoProxyLibrary_isProxyRunning() C.int {
	serverMu.Lock()
	defer serverMu.Unlock()
	if serverRunning {
		return 1
	}
	return 0
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
		fmt.Fprintf(w, `{"status": "healthy", "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
	})
}

func init() {}
