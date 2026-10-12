job "catalog" {
  datacenters = ["dc1"]
  type        = "service"

  group "catalog" {
    network {
      port "http" {
        to = 8080
      }
      port "grpc" {
        to = 9090
      }
    }

    task "catalog" {
      driver = "docker"

      config {
        image = "ghcr.io/matthiew/warpfactory:latest"
        ports = ["http", "grpc"]
        args  = ["-config", "/local/config.json"]
      }

      template {
        destination   = "local/config.json"
        change_mode   = "signal"
        change_signal = "SIGHUP"
        data          = <<EOF
{{ with nomadVar "nomad/jobs/catalog/catalog/catalog" }}{"catalog_label": {{ .catalog_label | toJSON }}}{{ end }}
EOF
      }

      resources {
        cpu    = 100
        memory = 128
      }
    }
  }
}
