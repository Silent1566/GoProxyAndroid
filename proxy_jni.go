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

//export Java_com_github_catvod_spider_GoProxyLibrary_startProxy
func Java_com_github_catvod_spider_GoProxyLibrary_startProxy(cPort C.int) C.int {
	port := int(cPort)
	if port <= 0 {
		port = 5575
	}

	serverMu.Lock()
	if serverRunning {
		serverMu.Unlock()
		return C.int(1)
	}
	serverMu.Unlock()

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

//export Java_com_github_catvod_spider_GoProxyLibrary_isProxyRunning
func Java_com_github_catvod_spider_GoProxyLibrary_isProxyRunning() C.int {
	serverMu.Lock()
	defer serverMu.Unlock()
	if serverRunning {
		return 1
	}
	return 0
}

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
			log.Printf("播放错误: %v", err)
		}
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status": "healthy", "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
	})
}

func init() {}
