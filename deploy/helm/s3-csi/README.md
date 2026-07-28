# Container Storage Interface (CSI) for S3

[Container storage interface](https://kubernetes-csi.github.io/docs/) is an [industry standard](https://github.com/container-storage-interface/spec/blob/master/spec.md) that enables storage vendors to develop a plugin once and have it work across a number of container orchestration systems.

[S3](https://github.com/s3/s3) is a simple and highly scalable distributed file system, to store and serve billions of files fast!

See [s3-csi-driver](https://github.com/s3/s3-csi-driver) for the source for this CSI plugin.

## Installing

Add the Helm repository:

```bash
helm repo add s3-csi-driver https://s3.github.io/s3-csi-driver/helm
helm repo update
```
  
Install the chart. You will need to specify the location of the S3 filer URL by either running:

```bash
helm install --set s3Filer=<filerHost:port> my-s3-csi-driver s3-csi-driver/s3-csi-driver
```

Or by configuring a s3-overrides.yaml file containing (for example):

```yaml
# For a S3 instance running locally under the "s3" namespace - adjust for your configuration
s3Filer: "s3-filer.s3.svc.cluster.local:8888"
```

And running:

```bash
helm install my-s3-csi-driver s3-csi-driver/s3-csi-driver -f s3-overrides.yaml
```

## Usage

See [Testing](https://github.com/s3/s3-csi-driver#testing) on some usage examples.
