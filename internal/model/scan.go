package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
)

// WorkflowStep 定义了工作流中的一个具体步骤
type WorkflowStep struct {
	Name             string `json:"name"`               // 步骤的唯一名称, e.g., "subfinder_step"
	TaskType         string `json:"task_type"`          // Asynq任务类型, e.g., "discovery:subdomain:subfinder"
	CommandTemplate  string `json:"command_template"`   // 命令模板, e.g., "subfinder -d {{.Input}} -json"
	InputFrom        string `json:"input_from"`         // "initial" 或上一个步骤的Name, 表示输入来源
	OutputParserType string `json:"output_parser_type"` // "subfinder_json", 指示用哪个解析器
	ExecutionMode    string `json:"execution_mode,omitempty"`
}

// WorkflowSteps 是 WorkflowStep 的切片，我们需要为它实现 GORM 的 Scanner/Valuer 接口
type WorkflowSteps []WorkflowStep

// Value - 实现 Valuer 接口, 告诉GORM如何将 WorkflowSteps 存入数据库
func (ws WorkflowSteps) Value() (driver.Value, error) {
	if len(ws) == 0 {
		return nil, nil
	}
	return json.Marshal(ws)
}

// Scan - 实现 Scanner 接口, 告诉GORM如何从数据库读取数据并解析到 WorkflowSteps
func (ws *WorkflowSteps) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	if len(bytes) == 0 {
		*ws = make(WorkflowSteps, 0)
		return nil
	}
	return json.Unmarshal(bytes, ws)
}

// ScanProfile 是一个可配置的扫描工作流模板
type ScanProfile struct {
	gorm.Model
	Name          string        `gorm:"unique;size:100;not null"`
	Description   string        `gorm:"type:text"`
	WorkflowSteps WorkflowSteps `gorm:"type:jsonb;not null"` // 使用JSONB存储工作流步骤数组
	IsActive      bool          `gorm:"default:true"`
}
