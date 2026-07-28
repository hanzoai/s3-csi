# Example of using s3 with HashiCorp Nomad


## Running s3 cluster

You can skip this part if you have already running s3.

Assumptions:
 - Running Nomad cluster
 - At least 3 nodes with static IP addresses
 - Enabled memory oversubscription (https://learn.hashicorp.com/tutorials/nomad/memory-oversubscription?in=nomad%2Fadvanced-scheduling)
 - Running PostgreSQL instance for filer

```shell
export NOMAD_ADDR=http://nomad.service.consul:4646

nomad run s3.hcl
```

S3 master will be available on http://s3-master.service.consul:9333/

S3 filer will be available on http://s3-filer.service.consul:8888/


## Running CSI

The CSI driver is split into two components that register under the same
`csi_plugin` id (`s3`):

 - **controller** (`s3-csi-controller.hcl`) — a single `service` job that
   implements the volume lifecycle RPCs. Nomad calls it for `nomad volume create`
   / `nomad volume delete`. It only talks to the filer.
 - **node** (`s3-csi.hcl`) — a `system` job that runs on every worker and
   stages/publishes volumes into allocations, using the `s3-mount`
   sidecar over a shared unix socket.

You need **both** running. With only the node plugin, `nomad volume create`
fails with `plugin has no controller`.

```shell
export NOMAD_ADDR=http://nomad.service.consul:4646

# Start CSI controller (one instance) and node (one per worker)
nomad run s3-csi-controller.hcl
nomad run s3-csi.hcl

# Wait until the plugin reports a healthy controller and the expected node count
nomad plugin status s3

# Create volume
nomad volume create example-s3-volume.hcl

# Start sample app
nomad run example-s3-app.hcl
```

> If you only run the node plugin (no controller), you cannot `nomad volume
> create`. Instead, pre-create the bucket/directory in S3 yourself and
> `nomad volume register example-s3-volume.hcl` to register the existing
> volume.
