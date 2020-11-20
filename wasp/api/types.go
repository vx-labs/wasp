package api

//go:generate protoc -I ${GOPATH}/src/github.com/vx-labs/wasp/v4/vendor -I ${GOPATH}/src/github.com/vx-labs/wasp/v4/vendor/github.com/gogo/protobuf/ -I ${GOPATH}/src/github.com/vx-labs/wasp/v4/wasp/api api.proto --go_out=plugins=grpc:.
