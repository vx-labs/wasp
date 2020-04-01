package main

import (
	"fmt"
	"math/rand"

	consul "github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

func findServer(l *zap.Logger, service, tag string) (string, error) {
	l.Debug("discovering Wasp server using Consul", zap.String("consul_service_name", service), zap.String("consul_service_tag", tag))
	client, err := consul.NewClient(consul.DefaultConfig())
	if err != nil {
		return "", err
	}
	services, _, err := client.Catalog().Service(service, tag, &consul.QueryOptions{})
	if err != nil {
		return "", err
	}
	var idx int
	if len(services) > 1 {
		l.Debug("discovered multiple Wasp servers using Consul", zap.Int("wasp_server_count", len(services)))
		idx = rand.Intn(len(services))
	} else if len(services) == 1 {
		idx = 0
	} else {
		l.Fatal("no Wasp server discovered using Consul")
	}
	return fmt.Sprintf("%s:%d", services[idx].ServiceAddress, services[idx].ServicePort), nil
}
