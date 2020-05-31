package main

import (
	"context"
	"errors"

	"github.com/spf13/viper"
	"github.com/vx-labs/wasp/wasp/audit"
	"github.com/vx-labs/wasp/wasp/rpc"
)

func getAuditRecorder(ctx context.Context, rpcDialer rpc.Dialer, config *viper.Viper) (audit.Recorder, error) {
	provider := config.GetString("audit-recorder")
	switch provider {
	case "none":
		return audit.NoneRecorder(), nil
	case "stdout":
		return audit.StdoutRecorder(), nil
	default:
		return nil, errors.New("unknown authentication provided")
	}
}
