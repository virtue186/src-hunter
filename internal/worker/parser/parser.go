package parser

import "github.com/src-hunter/internal/model"

// ParseResult 封装了解析后的标准化数据
type ParseResult struct {
	Domains []model.Domain // 解析出的域名
	Assets  []model.Asset  // 解析出的资产 (IP+端口)
}

// Parser 是所有输出解析器都必须实现的接口
type Parser interface {
	// Parse 接受命令的原始输出，返回一个标准化的ParseResult
	Parse(output []byte) (*ParseResult, error)
}
