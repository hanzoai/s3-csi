job "s3-csi-node" {
  datacenters = ["dc1"]
  type        = "system"
  priority    = 90

  constraint {
    attribute = "${meta.role}"
    value     = "worker"
  }

  update {
    max_parallel = 1
    stagger      = "45s"
  }

  group "nodes" {

    # directory to share the s3-mount unix socket between tasks
    volume "s3_mount_socket" {
      type      = "host"
      source    = "s3_mount_socket"
      read_only = false
    }

    # host directory where Nomad CSI stage/publish paths live
    volume "nomad_csi_s3" {
      type      = "host"
      source    = "nomad_csi_s3"
      read_only = false
    }

    ephemeral_disk {
      migrate = false
      size    = 1152
      sticky  = false
    }

    task "mount-service" {
      driver = "docker"

      volume_mount {
        volume      = "s3_mount_socket"
        destination = "/var/lib/s3-mount"
        read_only   = false
      }

      # required for proper propagation of staged mounts into the host filesystem
      volume_mount {
        volume           = "nomad_csi_s3"
        destination      = "/csi-data"
        read_only        = false
        propagation_mode = "bidirectional"
      }

      config {
        image        = "chrislusf/s3-mount:dev"
        network_mode = "host"
        privileged   = true
        args = [
          "--endpoint=unix:///var/lib/s3-mount/s3-mount.sock",
        ]
      }

      resources {
        cpu        = 200
        memory     = 1024
        memory_max = 2048
      }
    }

    task "plugin" {
      driver = "docker"

      volume_mount {
        volume      = "s3_mount_socket"
        destination = "/var/lib/s3-mount"
        read_only   = false
      }

      config {
        image        = "chrislusf/s3-csi-driver:v1.3.9"
        force_pull   = true
        network_mode = "host"
        privileged   = true
        args = [
          "--endpoint=unix:///csi-sock/csi.sock",
          "--filer=s3-filer.service.consul:8888",
          "--dataCenter=${node.datacenter}",
          "--nodeid=${node.unique.name}",
          "--cacheCapacityMB=1024",
          "--cacheDir=${NOMAD_ALLOC_DIR}/cache_dir",
          "--mountEndpoint=unix:///var/lib/s3-mount/s3-mount.sock",
          "--dataLocality=none",
          "--components=node",
        ]
      }

      csi_plugin {
        id                     = "s3"
        type                   = "node"
        mount_dir              = "/csi-sock"
        stage_publish_base_dir = "/csi-data/node/s3"
      }

      resources {
        cpu        = 800
        memory     = 512
        memory_max = 1024
      }
    }
  }
}
