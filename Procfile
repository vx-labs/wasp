0: sleep infinity # allow to stop and start the other nodes without goreman exiting
# You must create run_config directory and run "wasp tls" before starting TLS-enabled cluster.

1: PSK_PASSWORD="test" go run ./cmd/wasp/ -t 1883 --data-dir /tmp/wasp1 --debug -n 3 --serf-port 1790 --raft-port $PORT -j 127.0.0.1:1790 -j 127.0.0.1:1792 --rpc-tls-certificate-file ./run_config/cert.pem --rpc-tls-private-key-file ./run_config/privkey.pem --rpc-tls-certificate-authority-file ./run_config/cert.pem --mtls --metrics-port 8089
2: PSK_PASSWORD="test" go run ./cmd/wasp/ -t 1884 --data-dir /tmp/wasp2 --debug -n 3 --serf-port 1791 --raft-port $PORT -j 127.0.0.1:1790 -j 127.0.0.1:1792 --rpc-tls-certificate-file ./run_config/cert.pem --rpc-tls-private-key-file ./run_config/privkey.pem --rpc-tls-certificate-authority-file ./run_config/cert.pem --mtls
3: PSK_PASSWORD="test" go run ./cmd/wasp/ -t 1885 --data-dir /tmp/wasp3 --debug -n 3 --serf-port 1792 --raft-port $PORT -j 127.0.0.1:1790 -j 127.0.0.1:1791 --rpc-tls-certificate-file ./run_config/cert.pem --rpc-tls-private-key-file ./run_config/privkey.pem --rpc-tls-certificate-authority-file ./run_config/cert.pem --mtls
