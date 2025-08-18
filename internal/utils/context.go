package utils

import "context"

type ExecOptions struct {
	Verbose bool
	Quiet   bool
	DryRun  bool
}

type ctxKey int

const execOptsKey ctxKey = 1

func WithExecOptions(ctx context.Context, opts ExecOptions) context.Context {
	return context.WithValue(ctx, execOptsKey, opts)
}
func GetExecOptions(ctx context.Context) ExecOptions {
	if v, ok := ctx.Value(execOptsKey).(ExecOptions); ok {
		return v
	}
	return ExecOptions{}
}
