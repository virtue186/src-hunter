package parser

import (
	"bufio"
	"bytes"
	"encoding/json"
	"github.com/src-hunter/internal/model"
)

// SubfinderParser 负责解析 subfinder 的输出
type SubfinderParser struct{}

// 在包加载时，自动将自己注册到全局注册表
func init() {
	// 对应 ScanProfile 中的 "output_parser_type"
	Register("subfinder_json_list", &SubfinderParser{})
}

// subfinderOutputLine 是 subfinder -json 输出的每一行结构
type subfinderOutputLine struct {
	Host   string `json:"host"`
	Source string `json:"source"`
}

// Parse 实现了 Parser 接口
func (p *SubfinderParser) Parse(output []byte) (*ParseResult, error) {
	var domains []model.Domain
	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		var line subfinderOutputLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		if line.Host != "" {
			domains = append(domains, model.Domain{
				FQDN:   line.Host,
				Source: line.Source, // 记录发现来源
			})
		}
	}

	return &ParseResult{Domains: domains}, scanner.Err()
}
