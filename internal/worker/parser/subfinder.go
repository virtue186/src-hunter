package parser

import (
	"encoding/json"
	"github.com/src-hunter/internal/model"
)

// SubfinderParser 负责解析 subfinder 的输出
type SubfinderParser struct{}

func init() {
	// 对应 ScanProfile 中的 "output_parser_type"
	Register("subfinder_json_list", &SubfinderParser{})
}

// subfinderOutputLine 是 subfinder -json 输出的每一行结构
type subfinderOutputLine struct {
	Host   string `json:"host"`
	Source string `json:"source"`
}

// Parse 实现了 Parser 接口，现在可以直接解析一个包含多个对象的JSON数组
func (p *SubfinderParser) Parse(output []byte) (*ParseResult, error) {
	var lines []subfinderOutputLine
	// --- 核心修正点：直接将整个JSON数组反序列化到一个结构体切片中 ---
	if err := json.Unmarshal(output, &lines); err != nil {
		return nil, err
	}

	var domains []model.Domain
	for _, line := range lines {
		if line.Host != "" {
			domains = append(domains, model.Domain{
				FQDN:   line.Host,
				Source: line.Source,
			})
		}
	}

	return &ParseResult{Domains: domains}, nil
}
