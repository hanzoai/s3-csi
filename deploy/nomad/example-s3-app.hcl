job "example-s3-app" {
  datacenters = ["dc1"]

  group "apps" {
    volume "example-s3-volume" {
      type            = "csi"
      source          = "example-s3-volume"
      access_mode     = "multi-node-multi-writer"
      attachment_mode = "file-system"
    }

    task "sample" {
      driver = "docker"

      config {
        image      = "kadalu/sample-pv-check-app:latest"
        force_pull = false

        entrypoint = [
          "tail",
          "-f",
          "/dev/null",
        ]
      }

      volume_mount {
        volume = "example-s3-volume"
        destination = "/mnt/pv"
      }
    }
  }
}
