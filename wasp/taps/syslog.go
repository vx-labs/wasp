package taps

import (
	"context"
	"fmt"

	"log/syslog"

	"github.com/vx-labs/mqtt-protocol/packet"
)

func Syslog(ctx context.Context, remote string) (Tap, error) {
	conn, err := syslog.Dial("udp", remote, syslog.LOG_LOCAL0, "wasp")
	if err != nil {
		return nil, err
	}
	go func() {
		<-ctx.Done()
		defer conn.Close()
	}()
	return func(ctx context.Context, p *packet.Publish) error {
		_, err = conn.Write([]byte(fmt.Sprintf("%s <- %q", string(p.Topic), string(p.Payload))))
		return err
	}, nil
}
