package dto

import "time"

// DomainResponse 定义了单个域名信息的标准API响应结构
type DomainResponse struct {
	ID         uint      `json:"id"`
	FQDN       string    `json:"fqdn"`
	RootDomain string    `json:"rootDomain"`
	Source     string    `json:"source"`
	CreatedAt  time.Time `json:"createdAt"`
}
