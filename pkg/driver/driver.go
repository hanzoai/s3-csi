package driver

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/hanzoai/s3-csi/pkg/datalocality"
	"github.com/hanzoai/s3-csi/pkg/filer"
	"github.com/hanzoai/s3-csi/pkg/mountmanager"
	"github.com/hanzoai/s3/s3/glog"
	"github.com/hanzoai/s3/s3/pb"
	"github.com/hanzoai/s3/s3/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	version = "1.0.0"
)

type S3Driver struct {
	name    string
	nodeID  string
	version string

	endpoint        string
	mountEndpoint   string
	volumeSocketDir string // directory for volume sockets, derived from mountEndpoint

	vcap  []*csi.VolumeCapability_AccessMode
	cscap []*csi.ControllerServiceCapability

	filers     []pb.ServerAddress
	filerIndex int
	// zapFiler is the ONE storage client. It speaks ZAP to HanzoFiler on the
	// port historically named "grpc" (18888); hanzoai/s3 serves that port with
	// transport.ListenStream and registers no gRPC filer server at all.
	zapFiler          *filer.Client
	ConcurrentWriters int
	ConcurrentReaders int
	CacheCapacityMB   int
	CacheMetaTtlSec   int
	CacheDir          string
	UidMap            string
	GidMap            string
	signature         int32
	DataCenter        string
	DataLocality      datalocality.DataLocality

	RunNode       bool
	RunController bool
}

func NewS3Driver(name, filer, nodeID, endpoint, mountEndpoint string, enableAttacher bool) *S3Driver {

	glog.Infof("Driver: %v version: %v", name, version)

	util.LoadConfiguration("security", false)

	// Derive volumeSocketDir from mountEndpoint
	volumeSocketDir := mountmanager.DefaultSocketDir
	if mountEndpoint != "" {
		_, address, err := mountmanager.ParseEndpoint(mountEndpoint)
		if err != nil {
			glog.Warningf("invalid mount endpoint %q, using default socket directory %q: %v", mountEndpoint, volumeSocketDir, err)
		} else if address != "" {
			volumeSocketDir = filepath.Dir(address)
		}
	}

	n := &S3Driver{
		endpoint:        endpoint,
		mountEndpoint:   mountEndpoint,
		volumeSocketDir: volumeSocketDir,
		nodeID:          nodeID,
		name:            name,
		version:         version,
		filers:          pb.ServerAddresses(filer).ToAddresses(),
		signature:       util.RandomInt32(),
	}

	n.AddVolumeCapabilityAccessModes([]csi.VolumeCapability_AccessMode_Mode{
		csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
		csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		csi.VolumeCapability_AccessMode_SINGLE_NODE_MULTI_WRITER,
		csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER,
	})
	n.AddControllerServiceCapabilities([]csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
		csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
	})

	// we need this just only for csi-attach, but we do nothing for attach/detach
	if enableAttacher {
		n.AddControllerServiceCapabilities([]csi.ControllerServiceCapability_RPC_Type{
			csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
		})
	}

	return n
}

func (n *S3Driver) Run() {
	glog.Info("starting")

	var controller *ControllerServer
	if n.RunController {
		controller = NewControllerServer(n)
	}

	var node *NodeServer
	if n.RunNode {
		node = NewNodeServer(n)
	}

	s := NewNonBlockingGRPCServer()
	s.Start(n.endpoint,
		NewIdentityServer(n),
		controller,
		node)

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	<-stopChan

	glog.Infof("stopping")

	s.Stop()
	s.Wait()

	if node != nil {
		glog.Infof("node cleanup")
		node.NodeCleanup()
	}

	glog.Infof("stopped")
}

func (n *S3Driver) AddVolumeCapabilityAccessModes(vc []csi.VolumeCapability_AccessMode_Mode) {
	for _, c := range vc {
		glog.Infof("Enabling volume access mode: %v", c.String())
		n.vcap = append(n.vcap, &csi.VolumeCapability_AccessMode{Mode: c})
	}
}

func (n *S3Driver) AddControllerServiceCapabilities(cl []csi.ControllerServiceCapability_RPC_Type) {
	for _, c := range cl {
		glog.Infof("Enabling controller service capability: %v", c.String())
		n.cscap = append(n.cscap, NewControllerServiceCapability(c))
	}
}

func (d *S3Driver) ValidateControllerServiceRequest(c csi.ControllerServiceCapability_RPC_Type) error {
	if c == csi.ControllerServiceCapability_RPC_UNKNOWN {
		return nil
	}

	for _, cap := range d.cscap {
		if c == cap.GetRpc().GetType() {
			return nil
		}
	}
	return status.Error(codes.InvalidArgument, fmt.Sprintf("%s", c))
}

// Filer returns the ZAP filer client, dialing the first configured filer lazily.
// One client is shared: it is concurrency-safe and multiplexes calls over a
// single ZAP connection, so Create/Delete/Validate do not each open a socket.
func (d *S3Driver) Filer() *filer.Client {
	if d.zapFiler == nil {
		d.zapFiler = filer.New(d.filers[d.filerIndex].ToGrpcAddress())
	}
	return d.zapFiler
}

func (d *S3Driver) GetDataCenter() string {
	return d.DataCenter
}
