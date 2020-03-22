package wasp

import (
	"google.golang.org/grpc"
)

func ListenGRPC() *grpc.Server {
	return grpc.NewServer()
}
