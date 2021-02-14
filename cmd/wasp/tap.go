package main

import (
	"context"
	"errors"

	"github.com/spf13/viper"
	"github.com/vx-labs/wasp/v4/rpc"
	"github.com/vx-labs/wasp/v4/wasp/taps"
	"go.uber.org/zap"
)

func getTapRecorder(ctx context.Context, rpcDialer rpc.Dialer, config *viper.Viper, logger *zap.Logger) (taps.Tap, error) {
	provider := config.GetString("tap-recorder")
	switch provider {
	case "none":
		return nil, nil
	case "grpc":
		remote, err := rpcDialer(config.GetString("tap-recorder-grpc-address"))
		if err != nil {
			return nil, err
		}
		return taps.GRPC(remote)
	case "syslog":
		return taps.Syslog(ctx, config.GetString("tap-recorder-syslog-address"))
	case "stdout":
		return taps.Stdout()
	default:
		return nil, errors.New("unknown tap recorder provided")
	}
}
