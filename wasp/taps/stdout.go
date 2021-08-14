package taps

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/vx-labs/mqtt-protocol/packet"
)

func Stdout() (Tap, error) {
	return func(ctx context.Context, sender string, p *packet.Publish) error {
		if len(p.Payload) < 1200 && !bytes.ContainsAny(p.Payload, "\n") {
			fmt.Fprintf(os.Stdout, "%s <- %q\n", string(p.Topic), string(p.Payload))
		} else {
			fmt.Fprintf(os.Stdout, "%s <- %d bytes\n", string(p.Topic), len(p.Payload))
		}
		return nil
	}, nil
}
