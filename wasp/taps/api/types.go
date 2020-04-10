package api

//go:generate protoc -I ${GOPATH}/src/github.com/vx-labs/wasp/vendor -I ${GOPATH}/src/github.com/vx-labs/wasp/wasp/taps/api/ api.proto --go_out=plugins=grpc:.
