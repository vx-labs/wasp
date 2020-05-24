package auth

import (
	context "context"
)

type staticHandler struct {
	usernameHash string
	passwordHash string
}

func (h *staticHandler) Authenticate(ctx context.Context, mqtt ApplicationContext, transport TransportContext) (string, error) {
	if fingerprintBytes(mqtt.Username) != h.usernameHash || fingerprintBytes(mqtt.Password) != h.passwordHash {
		return "", ErrAuthenticationFailed
	}
	return DefaultMountPoint, nil
}

// StaticHandler returns a static authentication handler.
func StaticHandler(username, password string) (AuthenticationHandler, error) {
	return &staticHandler{
		usernameHash: fingerprintString(username),
		passwordHash: fingerprintString(password),
	}, nil
}
