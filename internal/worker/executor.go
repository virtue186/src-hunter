package worker

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// ExecutionResult 封装了命令执行的结果
type ExecutionResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// Executor 是一个可以执行具体命令的接口
type Executor interface {
	Run(ctx context.Context, command string, args ...string) (*ExecutionResult, error)
}

// LocalExecutor 实现了在本地机器上执行命令的逻辑
type LocalExecutor struct{}

func NewLocalExecutor() Executor {
	return &LocalExecutor{}
}

// Run 安全地执行一个外部命令，并处理超时
func (e *LocalExecutor) Run(ctx context.Context, command string, args ...string) (*ExecutionResult, error) {
	// 为每一个命令执行创建一个带超时的上下文
	// 这里的 5 分钟是一个默认值，未来可以从ScanProfile中读取
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &ExecutionResult{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		ExitCode: cmd.ProcessState.ExitCode(),
	}

	if ctx.Err() == context.DeadlineExceeded {
		return result, fmt.Errorf("命令执行超时")
	}

	if err != nil {
		return result, fmt.Errorf("命令执行失败: %w", err)
	}

	return result, nil
}
