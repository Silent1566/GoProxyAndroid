// proxy.go - 基于原版优化
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"
)

type Player struct {
	client    *http.Client
	header    http.Header
	start     int64
	end       int64
	thread    int
	chunkSize int64
	url       string
}

func NewPlayer(header http.Header, thread, chunkSizeKB int, url string) *Player {
	h := http.Header{}
	for _, key := range []string{"User-Agent", "Cookie", "Referer"} {
		if v := header.Get(key); v != "" {
			h.Set(key, v)
		}
	}
	start, end := parseRange(header.Get("Range"))

	return &Player{
		client: &http.Client{
			Timeout: 0, // 关键修复：移除全局超时
			Transport: &http.Transport{
				TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
				MaxIdleConns:          100,
				MaxIdleConnsPerHost:   20,
				IdleConnTimeout:       60 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
				DisableKeepAlives:     false,
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
			},
		},
		header:    h,
		start:     start,
		end:       end,
		thread:    thread,
		chunkSize: int64(chunkSizeKB) * 1024,
		url:       url,
	}
}

func (p *Player) Play(w http.ResponseWriter, ctx context.Context) error {
	// 下载第一个块
	s, e, err := p.downloadFirst(w, ctx)
	if err != nil {
		return err
	}
	fileSize := e + 1
	log.Printf("文件大小: %d MB, 线程: %d, 块大小: %d KB",
		fileSize/1024/1024, p.thread, p.chunkSize/1024)

	flusher, _ := w.(http.Flusher)
	if flusher != nil {
		flusher.Flush()
	}

	results := make([][]byte, p.thread)
	var wg sync.WaitGroup

	for start := s; start < fileSize; start += int64(p.chunkSize) * int64(p.thread) {
		// 检查取消
		select {
		case <-ctx.Done():
			log.Printf("请求被取消")
			return ctx.Err()
		default:
		}

		activeThreads := 0
		// 用于接收 goroutine 中的错误
		chunkErrors := make([]error, p.thread)

		for i := 0; i < p.thread; i++ {
			chunkStart := start + int64(i)*p.chunkSize
			chunkEnd := chunkStart + p.chunkSize
			if chunkStart >= fileSize {
				break
			}
			if chunkEnd > fileSize {
				chunkEnd = fileSize
			}

			results[i] = nil
			chunkErrors[i] = nil
			activeThreads++
			wg.Add(1)

			go func(idx int, cs, ce int64) {
				defer wg.Done()

				downloadCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
				defer cancel()

				// 添加重试逻辑
				var data []byte
				var err error
				for retry := 0; retry < 3; retry++ {
					data, _, _, err = p.downloadChunk(downloadCtx, cs, ce, 3)
					if err == nil {
						break
					}
					log.Printf("块 %d-%d 第%d次重试", cs, ce-1, retry+1)
					time.Sleep(time.Second * time.Duration(retry+1))
				}

				if err != nil {
					log.Printf("⚠️ 块 %d-%d 下载彻底失败: %v", cs, ce-1, err)
					chunkErrors[idx] = fmt.Errorf("数据块 %d (%d-%d) 下载失败: %w", idx, cs, ce-1, err)
					return
				}
				results[idx] = data
			}(i, chunkStart, chunkEnd)
		}

		// 等待当前批次完成
		wg.Wait()

		// 检查是否有块下载失败，重试全部失败后抛出错误
		for i := 0; i < activeThreads; i++ {
			if chunkErrors[i] != nil {
				log.Printf("❌ %v", chunkErrors[i])
				return chunkErrors[i] // 中止传输，不返回异常数据流
			}
		}

		// 检查是否被取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 写入当前批次的数据
		for i := 0; i < activeThreads; i++ {
			_, err = w.Write(results[i])
			if err != nil {
				log.Printf("写入失败: %v", err)
				return err
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
	}

	log.Printf("下载完成")
	return nil
}

func (p *Player) downloadFirst(w http.ResponseWriter, ctx context.Context) (int64, int64, error) {
	start, end := p.start, p.end
	if end <= 0 {
		end = 100
	} else {
		end += 1
	}
	end = start + min(end, p.chunkSize)

	chunk, header, status, err := p.downloadChunk(ctx, start, end, 3)
	if err != nil {
		return 0, 0, err
	}

	matches := crRegex.FindStringSubmatch(header.Get("Content-Range"))
	if len(matches) != 4 {
		return 0, 0, errors.New("未获取到文件总大小")
	}
	totalLength, _ := strconv.ParseInt(matches[3], 10, 64)

	if p.end <= 0 {
		end = totalLength - 1
	} else {
		end = p.end
	}

	h := w.Header()
	h.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, totalLength))
	for k, v := range header {
		if k != "Content-Range" && k != "Content-Length" {
			h[k] = v
		}
	}
	w.WriteHeader(status)

	_, err = w.Write(chunk)
	if err != nil {
		return 0, 0, err
	}

	return start + int64(len(chunk)), end, nil
}

func (p *Player) downloadChunk(ctx context.Context, start, end int64, maxRetries int) ([]byte, http.Header, int, error) {
	var lastErr error
	for retry := 0; retry < maxRetries; retry++ {
		req, err := http.NewRequestWithContext(ctx, "GET", p.url, nil)
		if err != nil {
			return nil, nil, -1, err
		}
		req.Header = p.header.Clone()
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end-1))

		resp, err := p.client.Do(req)
		if err != nil {
			lastErr = err
			if retry < maxRetries-1 {
				select {
				case <-time.After(time.Duration(retry+1) * 500 * time.Millisecond):
					continue
				case <-ctx.Done():
					return nil, nil, -1, ctx.Err()
				}
			}
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == 206 || resp.StatusCode == 200 {
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				lastErr = err
				continue
			}
			return data, resp.Header, resp.StatusCode, nil
		}

		lastErr = fmt.Errorf("状态码: %d", resp.StatusCode)
	}

	return nil, nil, -1, fmt.Errorf("重试%d次失败: %v", maxRetries, lastErr)
}

var crRegex = regexp.MustCompile(`bytes\s+(\d+)-(\d+)/(\d+)`)
var seRegex = regexp.MustCompile(`bytes=(\d+)-(\d*)`)

func parseRange(rangeStr string) (int64, int64) {
	match := seRegex.FindStringSubmatch(rangeStr)
	if len(match) == 0 {
		return 0, -1
	}
	start, _ := strconv.ParseInt(match[1], 10, 64)
	end := int64(-1)
	if match[2] != "" {
		end, _ = strconv.ParseInt(match[2], 10, 64)
	}
	return start, end
}
