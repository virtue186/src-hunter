package validator

import (
	"fmt"
	"net"
	"regexp"
)

// 定义一个正则表达式用于校验域名
// 这个表达式能覆盖大多数常见情况，但不是100%完美的RFC标准实现
var domainRegex = regexp.MustCompile(`^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

// ValidateTargetValue 根据目标类型，校验其值的合法性
func ValidateTargetValue(targetType, value string) error {
	switch targetType {
	case "ip":
		if net.ParseIP(value) == nil {
			return fmt.Errorf("'%s' 不是一个合法的IPv4或IPv6地址", value)
		}
	case "cidr":
		_, _, err := net.ParseCIDR(value)
		if err != nil {
			return fmt.Errorf("'%s' 不是一个合法的CIDR地址块: %w", value, err)
		}
	case "domain":
		if !domainRegex.MatchString(value) {
			return fmt.Errorf("'%s' 不是一个格式合法的域名", value)
		}
	default:
		return fmt.Errorf("不支持的目标类型 '%s'", targetType)
	}
	return nil
}
