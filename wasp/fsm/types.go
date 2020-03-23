package fsm

//go:generate protoc -I${GOPATH}/src -I${GOPATH}/src/github.com/vx-labs/wasp/wasp/fsm/ --go_out=plugins=grpc:. types.proto
