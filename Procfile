0: sleep infinity # allow to stop and start the other nodes without goreman exiting
1: PSK_PASSWORD="test" go run ./cmd/wasp/ -t 1883 --data-dir /tmp/wasp1 --debug -n 3 --serf-port 1790 --raft-port $PORT -j 127.0.0.1:1790 -j 127.0.0.1:1792
2: PSK_PASSWORD="test" go run ./cmd/wasp/ -t 1884 --data-dir /tmp/wasp2 --debug -n 3 --serf-port 1791 --raft-port $PORT -j 127.0.0.1:1790 -j 127.0.0.1:1792
3: PSK_PASSWORD="test" go run ./cmd/wasp/ -t 1885 --data-dir /tmp/wasp3 --debug -n 3 --serf-port 1792 --raft-port $PORT -j 127.0.0.1:1790 -j 127.0.0.1:1791
