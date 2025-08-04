package dto

import "time"

type TaskResponse struct {
	ID            uint      `json:"id"`
	ProjectID     uint      `json:"projectId"`
	ScanProfileID uint      `json:"scanProfileId"`
	Status        string    `json:"status"`
	Result        string    `json:"result"`
	FinishedAt    time.Time `json:"finishedAt"`
	CreatedAt     time.Time `json:"createdAt"`
}
