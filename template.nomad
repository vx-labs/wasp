job "wasp" {
  datacenters = ["dc1"]
  type        = "service"

  update {
    max_parallel     = 1
    min_healthy_time = "30s"
    healthy_deadline = "3m"
    health_check     = "task_states"
    auto_revert      = true
    canary           = 0
  }

  group "tcp-listener" {
    vault {
      policies      = ["nomad-tls-storer"]
      change_mode   = "signal"
      change_signal = "SIGUSR1"
      env           = false
    }

    count = 1

    restart {
      attempts = 10
      interval = "5m"
      delay    = "15s"
      mode     = "delay"
    }

    ephemeral_disk {
      size = 300
    }

    task "tcp-listener" {
      driver = "docker"

      env {
        CONSUL_HTTP_ADDR = "$${NOMAD_IP_health}:8500"
        VAULT_ADDR       = "http://active.vault.service.consul:8200/"
      }

      template {
        destination = "local/proxy.conf"
        env         = true

        data = <<EOH
{{with secret "secret/data/vx/mqtt"}}
http_proxy="{{.Data.http_proxy}}"
https_proxy="{{.Data.http_proxy}}"
LE_EMAIL="{{.Data.acme_email}}"
PSK_PASSWORD="{{ .Data.static_tokens }}"
JWT_SIGN_KEY="{{ .Data.jwt_sign_key }}"
TLS_CERTIFICATE="{{ env "NOMAD_TASK_DIR" }}/cert.pem"
TLS_PRIVATE_KEY="{{ env "NOMAD_TASK_DIR" }}/key.pem"
TLS_CA_CERTIFICATE="{{ env "NOMAD_TASK_DIR" }}/ca.pem"
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
{{- $ipsans := printf "ip_sans=%s" (env "NOMAD_IP_health") -}}
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
{{- $ipsans := printf "ip_sans=%s" (env "NOMAD_IP_health") -}}
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
{{- $ipsans := printf "ip_sans=%s" (env "NOMAD_IP_health") -}}
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

        image      = "${service_image}"
        args       = ["--tcp-port", "1883", "--use-vault", "--wss-port", "443", "--tls-port", "8883", "--tls-cn", "broker.iot.cloud.vx-labs.net"]
        force_pull = true

        port_map {
          mqtt   = 1883
          mqtts   = 8883
          mqttwss   = 443
        }
      }

      resources {
        cpu    = 200
        memory = 64

        network {
          mbits = 10
          port  "mqtt"{}
          port  "mqtts"{}
          port  "mqttwss"{}
        }
      }

      service {
        name = "mqtt"
        port = "mqtt"

        tags = [
          "traefik.enable=true",
          "traefik.tcp.routers.mqtt.rule=HostSNI(`*`)",
          "traefik.tcp.routers.mqtt.entrypoints=mqtt",
          "traefik.tcp.routers.mqtt.service=mqtt",
        ]

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
        tags = [
          "traefik.enable=true",
          "traefik.tcp.routers.mqtts.rule=HostSNI(`broker.iot.cloud.vx-labs.net`)",
          "traefik.tcp.routers.mqtts.entrypoints=mqtts",
          "traefik.tcp.routers.mqtts.service=mqtts",
          "traefik.tcp.routers.mqtts.tls",
          "traefik.tcp.routers.mqtts.tls.passthrough=true",
        ]

        check {
          type     = "tcp"
          port     = "mqtts"
          interval = "30s"
          timeout  = "2s"
        }
      }
    }
  }
}
