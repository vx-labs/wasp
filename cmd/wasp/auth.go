package main

import (
	"context"
	"errors"

	"github.com/spf13/viper"
	"github.com/vx-labs/wasp/v4/rpc"
	"github.com/vx-labs/wasp/v4/wasp/auth"
)

func getAuthHandler(ctx context.Context, rpcDialer rpc.Dialer, config *viper.Viper) (auth.AuthenticationHandler, error) {
	provider := config.GetString("authentication-provider")
	switch provider {
	case "none":
		return auth.NoopHandler(), nil
	case "static":
		return auth.StaticHandler(
			config.GetString("authentication-provider-static-username"),
			config.GetString("authentication-provider-static-password"),
		)
	case "file":
		return auth.FileHandler(
			config.GetString("authentication-provider-file-path"),
		)
	case "grpc":
		host := config.GetString("authentication-provider-grpc-address")
		conn, err := rpcDialer(host)
		if err != nil {
			return nil, err
		}
		return auth.GRPC(
			conn,
		)
	default:
		return nil, errors.New("unknown authentication provided")
	}
}
