package model

import (
	"gorm.io/gorm"
	"time"
)

type Project struct {
	gorm.Model
	Name        string `gorm:"unique;size:255;not null;comment:项目名称,必须唯一"`
	Description string `gorm:"type:text;comment:项目描述"`
	Status      string `gorm:"size:50;default:'active';index;comment:项目状态 (active, archived)"`
}

type ProjectTarget struct {
	gorm.Model
	ProjectID uint `gorm:"uniqueIndex:idx_project_target_unique;comment:所属项目ID"`
	// 核心字段
	Value string `gorm:"uniqueIndex:idx_project_target_unique;size:1024;not null;comment:目标的值 (e.g., example.com, 1.2.3.4, 10.0.0.0/8)"`
	Type  string `gorm:"size:50;not null;comment:目标类型 (domain, ip, cidr)"`
	// 扩展字段
	Description string `gorm:"type:text;comment:对此目标的描述"`
	IsActive    bool   `gorm:"default:true;index;comment:是否启用对此目标的周期性扫描"`
}

type Asset struct {
	gorm.Model
	ProjectID uint `gorm:"uniqueIndex:idx_asset_unique_in_project;comment:所属项目ID"`

	// 唯一确定一个资产服务
	IP   string `gorm:"uniqueIndex:idx_asset_unique_in_project;size:128;comment:IPv4或IPv6地址"`
	Port int    `gorm:"uniqueIndex:idx_asset_unique_in_project;comment:端口号"`

	Protocol   string    `gorm:"size:50;comment:应用层协议 (e.g., http, ssh)"`
	Source     string    `gorm:"size:100;comment:发现来源 (e.g., nmap, masscan)"`
	LastSeenAt time.Time `gorm:"index;comment:最后一次扫描到此资产存活的时间"`
}

type Domain struct {
	gorm.Model
	ProjectID uint `gorm:"uniqueIndex:idx_domain_unique_in_project;comment:所属项目ID"`

	FQDN       string    `gorm:"uniqueIndex:idx_domain_unique_in_project;size:255;comment:完整域名"`
	RootDomain string    `gorm:"index;size:255;comment:根域名, 用于聚合查询"`
	Source     string    `gorm:"size:100;comment:发现来源 (e.g., subfinder)"`
	LastSeenAt time.Time `gorm:"index;comment:最后一次解析到此域名的时间"`
}

type AssetDomainMapping struct {
	AssetID  uint `gorm:"primaryKey"`
	DomainID uint `gorm:"primaryKey"`
}

type IPMetadata struct {
	IP           string `gorm:"primaryKey;size:128"`
	UpdatedAt    time.Time
	ASN          string `gorm:"size:100;comment:自治系统编号"`
	Organization string `gorm:"size:255;comment:所属组织"`
	CountryCode  string `gorm:"size:10;comment:国家代码"`
	Source       string `gorm:"size:100;comment:数据来源 (e.g., geoip)"`
}

type Task struct {
	gorm.Model
	ProjectID     uint      `gorm:"index;comment:任务所属的项目ID"`
	ScanProfileID uint      `gorm:"index;comment:关联的扫描模板ID"` // <-- 新增字段
	AsynqID       string    `gorm:"index;size:128;comment:Asynq任务的唯一ID,父任务或独立任务才有"`
	Type          string    `gorm:"index;size:100;comment:任务类型"`
	Payload       []byte    `gorm:"type:jsonb;comment:任务载荷(JSON格式)"`
	Queue         string    `gorm:"index;size:50;comment:所属队列"`
	Status        string    `gorm:"index;size:50;comment:任务状态 (pending, running, success, failed)"`
	Result        string    `gorm:"type:text;comment:任务执行结果或错误信息"`
	StartedAt     time.Time `gorm:"comment:任务开始执行时间"`
	FinishedAt    time.Time `gorm:"comment:任务执行完毕时间"`

	ParentTaskID    uint   `gorm:"index;comment:父任务ID，用于工作流"`
	WorkflowStep    string `gorm:"size:100;comment:在工作流中所处的步骤名"`
	PendingSubtasks int    `gorm:"default:0;comment:扇出任务的待处理子任务数量"`
}
