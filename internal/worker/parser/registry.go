package parser

import "fmt"

// registry 是一个全局的map，用于存储所有已注册的解析器
var registry = make(map[string]Parser)

// Register 用于向注册表注册一个新的解析器
func Register(name string, p Parser) {
	if _, exists := registry[name]; exists {
		// 防止重复注册
		panic(fmt.Sprintf("解析器名称 '%s' 已被注册", name))
	}
	registry[name] = p
}

// Get 根据名称从注册表中获取一个解析器实例
func Get(name string) (Parser, error) {
	p, exists := registry[name]
	if !exists {
		return nil, fmt.Errorf("未找到名为 '%s' 的解析器", name)
	}
	return p, nil
}
