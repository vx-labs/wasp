package messages

//go:generate protoc -I ${GOPATH}/src/github.com/vx-labs/wasp/vendor -I ${GOPATH}/src/github.com/vx-labs/wasp/vendor/github.com/gogo/protobuf/ -I ${GOPATH}/src/github.com/vx-labs/wasp/wasp/messages messages.proto --go_out=plugins=grpc:.

import (
	"encoding/binary"
)

// Converts bytes to an integer
func bytesToUint64(b []byte) uint64 {
	if b == nil {
		return 0
	}
	return binary.BigEndian.Uint64(b)
}

// Converts a uint to a byte slice
func uint64ToBytes(u uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, u)
	return buf
}
