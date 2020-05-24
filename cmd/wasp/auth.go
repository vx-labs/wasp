package main

import (
	"context"
	"errors"

	"github.com/spf13/viper"
	"github.com/vx-labs/wasp/wasp/auth"
)

func getAuthHandler(ctx context.Context, config *viper.Viper) (auth.AuthenticationHandler, error) {
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
	default:
		return nil, errors.New("unknown authentication provided")
	}
}
