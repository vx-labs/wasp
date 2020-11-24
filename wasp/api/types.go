package api

//go:generate protoc -I ${GOPATH}/src/github.com/vx-labs/wasp/vendor -I ${GOPATH}/src/github.com/vx-labs/wasp/vendor/github.com/gogo/protobuf/ -I ${GOPATH}/src/github.com/vx-labs/wasp/wasp/api api.proto --go_out=plugins=grpc:.
