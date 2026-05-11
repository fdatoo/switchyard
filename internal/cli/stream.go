package cli

import (
	"context"

	"connectrpc.com/connect"
)

func liveStreamContextErr(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return renderConnectErr(connect.NewError(connect.CodeDeadlineExceeded, err))
	}
	if _, ok := ctx.Deadline(); ok {
		<-ctx.Done()
		return renderConnectErr(connect.NewError(connect.CodeDeadlineExceeded, ctx.Err()))
	}
	return nil
}
