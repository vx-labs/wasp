# Wasp

Distributed MQTT broker, written in Go.

## Running

The broker is composed of multiple nodes, discovering themselves using an embed gossip-based mesh, and building a raft cluster.

Inter-node communication is based on GRPC.

The project use Goreman (https://github.com/mattn/goreman/) to run on a local development workstation.

### Local

The system persists its state on disk. During development, you may have to delete the persisted state before starting the system.

Persisted state is located by default in the /tmp/wasp folder. You can (and should) change this location using the --data-dir flag.

You can run the broker on your workstation by running the following command.
```
goreman start
```

Wait a few seconds for raft clusters (key-value store, stream service and queues service) to bootstrap themselves.

The broker should start and listen on 0.0.0.0:1883 (tcp),0.0.0.0:1884 (tcp) and 0.0.0.0:1885 (tcp).
You can connect to it using an MQTT client, like Mosquitto.

```
mosquitto_sub -t 'test' -d -q 1 &
mosquitto_pub --port 1884 -t 'test' -d -q 1 -m 'hello'
```
