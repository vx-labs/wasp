1: go run ./cmd/wasp/ -t 1883 --data-dir /tmp/wasp1 --debug -n 3 --serf-port 1790  --raft-port $(( $PORT + 1 ))
2: go run ./cmd/wasp/ -t 1884 --data-dir /tmp/wasp2 --debug -n 3 --serf-port $PORT --raft-port $(( $PORT + 1 )) -j 127.0.0.1:1790
3: go run ./cmd/wasp/ -t 1885 --data-dir /tmp/wasp3 --debug -n 3 --serf-port $PORT --raft-port $(( $PORT + 1 )) -j 127.0.0.1:1790