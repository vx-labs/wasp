package taps

import (
	"context"
	"fmt"
	"os"

	"github.com/vx-labs/mqtt-protocol/packet"
)

func Stdout() (Tap, error) {
	return func(ctx context.Context, sender string, p *packet.Publish) error {
		fmt.Fprintf(os.Stdout, "%s <- %q\n", string(p.Topic), string(p.Payload))
		return nil
	}, nil
}
