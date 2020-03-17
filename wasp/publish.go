package wasp

import (
	"github.com/golang/protobuf/proto"
	"github.com/vx-labs/mqtt-protocol/packet"
)

func ProcessPublish(state State, p *packet.Publish) error {
	if p.Header.Retain {
		err := state.RetainMessage(p)
		if err != nil {
			return err
		}
		p.Header.Retain = false
	}
	recipients, qoss, err := state.Recipients(p.Topic, p.Header.Qos)
	if err != nil {
		return err
	}
	for idx := range recipients {
		session := state.GetSession(recipients[idx])
		if session != nil {
			p.Header.Qos = qoss[idx]
			session.Send(p)
		}
	}
	return nil
}
func StorePublish(messageLog MessageLog, p []*packet.Publish) error {
	buf := make([][]byte, len(p))
	for idx := range buf {
		payload, err := proto.Marshal(p[idx])
		if err != nil {
			return err
		}
		buf[idx] = payload
	}
	err := messageLog.Append(buf)
	if err != nil {
		return err
	}
	return nil
}
