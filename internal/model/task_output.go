package model

import (
	"database/sql/driver"
	"errors"
	"gorm.io/gorm"
)

// JSONB 是一个可以处理任意JSON数据的自定义类型，用于存储结构化结果。
type JSONB []byte

func (j JSONB) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return string(j), nil
}

func (j *JSONB) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	*j = bytes
	return nil
}

// TaskOutput 存储了单个任务执行完毕后的结构化输出结果。
// 它是工作流中步骤之间传递数据的“公告板”。
type TaskOutput struct {
	gorm.Model
	// 关联的子任务ID
	TaskID uint `gorm:"uniqueIndex;not null"`
	// 关联的父任务ID (工作流的顶级任务)
	ParentTaskID uint `gorm:"index"`
	// 输出数据的类型，便于后续处理，如 "subfinder_json_list"
	OutputType string `gorm:"size:100;not null"`
	// 存储结构化数据的JSONB字段
	Data JSONB `gorm:"type:jsonb"`
}
