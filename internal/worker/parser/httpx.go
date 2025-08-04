package parser

import (
	"bufio"
	"bytes"
	"encoding/json"
	"github.com/src-hunter/internal/model"
	"net/url"
	"strconv"
)

// HttpxParser 负责解析 httpx 的输出
type HttpxParser struct{}

func init() {
	Register("httpx_json_list", &HttpxParser{})
}

// httpxOutputLine 结构体与 httpx 的实际输出完全匹配
type httpxOutputLine struct {
	URL       string   `json:"url"`
	Title     string   `json:"title"`
	WebServer string   `json:"webserver"`
	Tech      []string `json:"tech"`
	A         []string `json:"a"`    // IPv4 地址列表
	Aaaa      []string `json:"aaaa"` // IPv6 地址列表
	Port      string   `json:"port"`
}

// Parse 实现了 Parser 接口，现在会为每个IP创建一条资产记录
func (p *HttpxParser) Parse(output []byte) (*ParseResult, error) {
	var assets []model.Asset
	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		var line httpxOutputLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}

		parsedURL, err := url.Parse(line.URL)
		if err != nil {
			continue
		}
		protocol := parsedURL.Scheme
		portInt, _ := strconv.Atoi(line.Port)

		// 将所有发现的IP地址收集到一个切片中
		allIPs := []string{}
		allIPs = append(allIPs, line.A...)
		allIPs = append(allIPs, line.Aaaa...)

		// --- 核心修正点在这里 ---
		// 遍历所有IP地址，为每一个IP都创建一个Asset记录
		for _, ip := range allIPs {
			// 跳过空的IP地址
			if ip == "" {
				continue
			}

			asset := model.Asset{
				IP:           ip, // 使用当前遍历到的IP
				Port:         portInt,
				Protocol:     protocol,
				Source:       "httpx",
				Title:        line.Title,
				WebServer:    line.WebServer,
				Technologies: line.Tech,
			}
			assets = append(assets, asset)
		}
		// --- 修正结束 ---
	}

	return &ParseResult{Assets: assets}, scanner.Err()
}
