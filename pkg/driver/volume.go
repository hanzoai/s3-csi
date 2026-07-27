package driver

import (
	"os"
	"sync"

	"github.com/hanzoai/s3-csi/pkg/mountmanager"
	"github.com/hanzoai/s3/s3/glog"
	mountwire "github.com/hanzoai/s3/s3/wire/mount"
	"k8s.io/mount-utils"
)

const volumeCapacityKey = "volumeCapacity"

type Volume struct {
	VolumeId   string
	StagedPath string

	mounter   Mounter
	unmounter Unmounter

	// unix socket used to manage volume
	localSocket string
	driver      *S3Driver

	// Fields for health monitor recovery
	publishPaths sync.Map          // targetPath (string) -> bool (readOnly)
	volContext   map[string]string // volume context stored for re-staging
	readOnly     bool              // FUSE-level readOnly flag

	// bindMountFn is used by Publish to perform the bind mount from the
	// staging path to the pod-specific target path. Populated by the
	// NodeServer that owns this volume; tests override it with a fake.
	bindMountFn BindMountFn
}

func NewVolume(volumeID string, mounter Mounter, driver *S3Driver) *Volume {
	return &Volume{
		VolumeId:    volumeID,
		mounter:     mounter,
		localSocket: mountmanager.LocalSocketPath(driver.volumeSocketDir, volumeID),
		driver:      driver,
	}
}

func (vol *Volume) Stage(stagingTargetPath string) error {
	// check whether it can be mounted
	if isMnt, err := checkMount(stagingTargetPath); err != nil {
		return err
	} else if isMnt {
		// try to unmount before mounting again
		_ = mountutil.Unmount(stagingTargetPath)
	}

	if u, err := vol.mounter.Mount(stagingTargetPath); err == nil {
		if vol.StagedPath != "" {
			if vol.StagedPath == stagingTargetPath {
				glog.Warningf("staged path is already set to %s for volume %s", vol.StagedPath, vol.VolumeId)
			} else {
				glog.Warningf("staged path is already set to %s and differs from %s for volume %s", vol.StagedPath, stagingTargetPath, vol.VolumeId)
			}
		}

		vol.StagedPath = stagingTargetPath
		vol.unmounter = u

		return nil
	} else {
		return err
	}
}

func (vol *Volume) Publish(stagingTargetPath string, targetPath string, readOnly bool) error {
	// check whether it can be mounted
	if isMnt, err := checkMount(targetPath); err != nil {
		return err
	} else if isMnt {
		// maybe already mounted?
		return nil
	}

	bind := vol.bindMountFn
	if bind == nil {
		bind = defaultBindMount
	}
	return bind(stagingTargetPath, targetPath, readOnly)
}

// defaultBindMount performs a real bind mount via mountutil. It is the
// production implementation of BindMountFn.
func defaultBindMount(source, target string, readOnly bool) error {
	mountOptions := []string{"bind"}
	if readOnly {
		mountOptions = append(mountOptions, "ro")
	}
	return mountutil.Mount(source, target, "", mountOptions)
}

func (vol *Volume) Quota(sizeByte int64) error {
	// The local mount's control channel is ZAP, not gRPC: hanzoai/s3 serves
	// HanzoMount over transport (s3/wire/mount) and generates no mount_pb Go
	// package at all. Dial the mount's unix socket directly — the
	// "passthrough:///unix://" form was a gRPC resolver artifact and is not
	// needed here.
	client, err := mountwire.Dial("unix", vol.localSocket)
	if err != nil {
		return err
	}
	defer client.Close()

	// A PV cannot be zero-sized, so a 1-byte quota is the encoding for "no
	// quota"; the mount expects 0 to mean unlimited.
	if sizeByte == 1 {
		sizeByte = 0
	}
	return client.Configure(sizeByte)
}

func (vol *Volume) Unpublish(targetPath string) error {
	// Try unmounting target path and deleting it.
	if err := mount.CleanupMountPoint(targetPath, mountutil, true); err != nil {
		return err
	}

	return nil
}

func (vol *Volume) AddPublishPath(path string, readOnly bool) {
	vol.publishPaths.Store(path, readOnly)
}

func (vol *Volume) RemovePublishPath(path string) {
	vol.publishPaths.Delete(path)
}

func (vol *Volume) Unstage(stagingTargetPath string) error {
	glog.V(0).Infof("unmounting volume %s from %s", vol.VolumeId, stagingTargetPath)

	if stagingTargetPath != vol.StagedPath && vol.StagedPath != "" {
		glog.Warningf("staging path %s differs for volume %s at %s", stagingTargetPath, vol.VolumeId, vol.StagedPath)
	}

	if vol.unmounter == nil {
		// This can happen when the volume was rebuilt from an existing staging path
		// after a CSI driver restart. In this case, we need to force unmount.
		glog.Infof("volume %s has no unmounter (rebuilt from existing mount), using force unmount", vol.VolumeId)

		// Clean up using mount utilities. This will also handle unmounting.
		if err := mount.CleanupMountPoint(stagingTargetPath, mountutil, true); err != nil {
			glog.Errorf("error cleaning up mount point for volume %s: %v", vol.VolumeId, err)
			return err
		}
	} else {
		if err := vol.unmounter.Unmount(); err != nil {
			glog.Errorf("error unmounting volume during unstage: %s, err: %v", stagingTargetPath, err)
			return err
		}

		if err := os.Remove(stagingTargetPath); err != nil && !os.IsNotExist(err) {
			glog.Errorf("error removing staging path for volume %s at %s, err: %v", vol.VolumeId, stagingTargetPath, err)
			return err
		}
	}

	// Always attempt to remove the cache directory and socket file
	CleanupVolumeResources(vol.driver, vol.VolumeId)

	return nil
}
