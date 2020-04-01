job "wasp" {
  datacenters = ["dc1"]
  type        = "service"

  update {
    max_parallel     = 1
    min_healthy_time = "30s"
    healthy_deadline = "3m"
    auto_revert      = true
    canary           = 0
  }

  group "broker" {
    vault {
      policies      = ["nomad-tls-storer"]
      change_mode   = "signal"
      change_signal = "SIGUSR1"
      env           = false
    }

    count = 3

    restart {
      attempts = 10
      interval = "5m"
      delay    = "15s"
      mode     = "delay"
    }

    ephemeral_disk {
      size = 2000
    }

    task "tcp-listener" {
      driver = "docker"

      env {
        CONSUL_HTTP_ADDR = "$${NOMAD_IP_rpc}:8500"
        VAULT_ADDR       = "http://active.vault.service.consul:8200/"
      }

      template {
        change_mode = "restart"
        destination = "local/environment"
        env         = true

        data = <<EOH
{{with secret "secret/data/vx/mqtt"}}
LE_EMAIL="{{.Data.acme_email}}"
PSK_PASSWORD="{{ .Data.static_tokens }}"
JWT_SIGN_KEY="{{ .Data.jwt_sign_key }}"
WASP_RPC_TLS_CERTIFICATE_FILE="{{ env "NOMAD_TASK_DIR" }}/cert.pem"
WASP_RPC_TLS_PRIVATE_KEY_FILE="{{ env "NOMAD_TASK_DIR" }}/key.pem"
WASP_RPC_TLS_CERTIFICATE_AUTHORITY_FILE="{{ env "NOMAD_TASK_DIR" }}/ca.pem"
no_proxy="10.0.0.0/8,172.16.0.0/12,*.service.consul"
{{end}}
        EOH
      }

      template {
        change_mode = "restart"
        destination = "local/cert.pem"
        splay       = "1h"

        data = <<EOH
{{- $cn := printf "common_name=%s" (env "NOMAD_ALLOC_ID") -}}
{{- $ipsans := printf "ip_sans=%s" (env "NOMAD_IP_rpc") -}}
{{- $path := printf "pki/issue/grpc" -}}
{{ with secret $path $cn $ipsans "ttl=48h" }}{{ .Data.certificate }}{{ end }}
EOH
      }

      template {
        change_mode = "restart"
        destination = "local/key.pem"
        splay       = "1h"

        data = <<EOH
{{- $cn := printf "common_name=%s" (env "NOMAD_ALLOC_ID") -}}
{{- $ipsans := printf "ip_sans=%s" (env "NOMAD_IP_rpc") -}}
{{- $path := printf "pki/issue/grpc" -}}
{{ with secret $path $cn $ipsans "ttl=48h" }}{{ .Data.private_key }}{{ end }}
EOH
      }

      template {
        change_mode = "restart"
        destination = "local/ca.pem"
        splay       = "1h"

        data = <<EOH
{{- $cn := printf "common_name=%s" (env "NOMAD_ALLOC_ID") -}}
{{- $ipsans := printf "ip_sans=%s" (env "NOMAD_IP_rpc") -}}
{{- $path := printf "pki/issue/grpc" -}}
{{ with secret $path $cn $ipsans "ttl=48h" }}{{ .Data.issuing_ca }}{{ end }}
EOH
      }

      config {
        logging {
          type = "fluentd"

          config {
            fluentd-address = "localhost:24224"
            tag             = "wasp-tcp-listener"
          }
        }

        image = "${service_image}:${service_version}"
        args = [
          "--data-dir", "$${NOMAD_TASK_DIR}",
          "--mtls",
          "--use-vault",
          "--raft-bootstrap-expect", "3",
          "--consul-join",
          "--consul-service-name", "wasp",
          "--consul-service-tag", "gossip",
          "--wss-port", "443", "--tls-port", "8883", "--tcp-port", "1883",
          "--metrics-port", "8089",
          "--tls-cn", "broker.iot.cloud.vx-labs.net",
          "--raft-advertized-address", "$${NOMAD_IP_rpc}", "--raft-advertized-port", "$${NOMAD_HOST_PORT_rpc}",
          "--serf-advertized-address", "$${NOMAD_IP_gossip}", "--serf-advertized-port", "$${NOMAD_HOST_PORT_gossip}",
        ]
        force_pull = true

        port_map {
          mqtt    = 1883
          mqtts   = 8883
          mqttwss = 443
          gossip  = 1799
          rpc     = 1899
          metrics = 8089
        }
      }

      resources {
        cpu    = 200
        memory = 256

        network {
          mbits = 10
          port "mqtt" {}
          port "mqtts" {}
          port "mqttwss" {}
          port "rpc" {}
          port "gossip" {}
          port "metrics" {}
        }
      }

      service {
        name = "mqtt"
        port = "mqtt"
        tags = ["${service_version}"]

        check {
          type     = "tcp"
          port     = "mqtt"
          interval = "30s"
          timeout  = "2s"
        }
      }
      service {
        name = "mqttwss"
        port = "mqttwss"

        tags = [
          "${service_version}",
          "traefik.enable=true",
          "traefik.tcp.routers.wss.rule=HostSNI(`broker.iot.cloud.vx-labs.net`)",
          "traefik.tcp.routers.wss.entrypoints=https",
          "traefik.tcp.routers.wss.service=mqttwss",
          "traefik.tcp.routers.wss.tls",
          "traefik.tcp.routers.wss.tls.passthrough=true",
        ]

        check {
          type     = "tcp"
          port     = "mqttwss"
          interval = "30s"
          timeout  = "2s"
        }
      }
      service {
        name = "mqtts"
        port = "mqtts"
        tags = ["${service_version}"]
        check {
          type     = "tcp"
          port     = "mqtts"
          interval = "30s"
          timeout  = "2s"
        }
      }
      service {
        name = "wasp"
        port = "rpc"
        tags = [
          "rpc",
          "${service_version}",
          "traefik.enable=true",
          "traefik.tcp.routers.rpc.rule=HostSNI(`rpc.iot.cloud.vx-labs.net`)",
          "traefik.tcp.routers.rpc.entrypoints=https",
          "traefik.tcp.routers.rpc.service=rpc",
          "traefik.tcp.routers.rpc.tls",
          "traefik.tcp.routers.rpc.tls.passthrough=true",
        ]

        check {
          type     = "tcp"
          port     = "rpc"
          interval = "30s"
          timeout  = "2s"
        }
      }
      service {
        name = "wasp"
        port = "gossip"
        tags = ["gossip", "${service_version}"]

        check {
          type     = "tcp"
          port     = "gossip"
          interval = "30s"
          timeout  = "2s"
        }
      }
      service {
        name = "wasp"
        port = "metrics"
        tags = ["prometheus", "${service_version}"]

        check {
          type     = "http"
          path     = "/metrics"
          port     = "metrics"
          interval = "30s"
          timeout  = "2s"
        }
      }
    }
  }
}
