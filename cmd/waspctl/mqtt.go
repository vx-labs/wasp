package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/api"
	"go.uber.org/zap"
)

func Mqtt(ctx context.Context, config *viper.Viper) *cobra.Command {
	mqtt := &cobra.Command{
		Use: "mqtt",
	}
	mqtt.AddCommand(&cobra.Command{
		Use: "list-sessions",
		Run: func(cmd *cobra.Command, _ []string) {
			conn, l := mustDial(ctx, cmd, config)
			out, err := api.NewMQTTClient(conn).ListSessionMetadatas(ctx, &api.ListSessionMetadatasRequest{})
			if err != nil {
				l.Fatal("failed to list connected sessions", zap.Error(err))
			}
			table := getTable([]string{"ID", "Client ID", "Peer", "Connected Since"}, cmd.OutOrStdout())
			for _, member := range out.GetSessionMetadatasList() {
				table.Append([]string{
					member.GetSessionID(),
					member.GetClientID(),
					fmt.Sprintf("%x", member.GetPeer()),
					time.Since(time.Unix(member.GetConnectedAt(), 0)).String(),
				})
			}
			table.Render()
		},
	})
	distributeMessage := &cobra.Command{
		Use: "distribute-message",
		Run: func(cmd *cobra.Command, _ []string) {
			conn, l := mustDial(ctx, cmd, config)
			var payload []byte
			if p := config.GetString("payload"); p != "" {
				payload = []byte(p)
			}
			_, err := api.NewMQTTClient(conn).DistributeMessage(ctx, &api.DistributeMessageRequest{
				ResolveRemoteRecipients: config.GetBool("resolve-remote-recipients"),
				Message: &packet.Publish{
					Header: &packet.Header{
						Dup:    config.GetBool("dup"),
						Qos:    config.GetInt32("qos"),
						Retain: config.GetBool("retain"),
					},
					Topic:   []byte(config.GetString("topic")),
					Payload: payload,
				},
			})
			if err != nil {
				l.Fatal("failed to distribute message", zap.Error(err))
			}
		},
	}
	distributeMessage.Flags().Bool("resolve-remote-recipients", true, "Distribute the message accross all servers instances")
	distributeMessage.Flags().Bool("dup", false, "Mark the message as duplicate.")
	distributeMessage.Flags().BoolP("retain", "r", false, "Mark the message as retained.")
	distributeMessage.Flags().Int32P("qos", "q", int32(0), "Set the Message QoS.")
	distributeMessage.Flags().StringP("topic", "t", "", "Set the Message topic.")
	distributeMessage.Flags().StringP("payload", "p", "", "Set the Message payload.")
	distributeMessage.MarkFlagRequired("topic")
	mqtt.AddCommand(distributeMessage)
	return mqtt
}
