// main.go - 移除panic恢复
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
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

		// 关键：不使用panic恢复，让错误正常返回
		if err := player.Play(w, r.Context()); err != nil {
			log.Printf("播放错误: %v", err)
			// 已经开始写入数据，不要再写错误响应
		}
	})

	// 添加健康检查端点
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status": "healthy", "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
	})

	log.SetOutput(os.Stdout)
	log.Printf("服务器启动在 :5575")
	log.Fatal(http.ListenAndServe(":5575", nil))
}
