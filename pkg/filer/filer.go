// Package filer is the driver's ZAP-native client for the HanzoFiler service.
//
// It replaces the upstream driver's protobuf/gRPC filer client. hanzoai/s3 serves
// HanzoFiler over the native ZAP transport on the port historically called the
// "gRPC port" (18888) — s3/command/filer.go binds it with transport.ListenStream
// and states plainly that this "replaces the legacy gRPC HanzoFiler server: the
// whole filer (28 unary + 5 streaming RPCs) now answers over ZAP, no gRPC".
// There is no RegisterHanzoFilerServer anywhere in that tree.
//
// So a gRPC client cannot talk to it at all — the failure mode is
// `Unimplemented: unknown service filer_pb.S3Filer`, which reads like a
// service-name mismatch but is really a gRPC client hitting a non-gRPC port. No
// amount of renaming or re-addressing fixes that; the transport has to change.
//
// Only three operations are needed to back CSI volumes, because a volume IS a
// directory under the filer root:
//
//	Mkdir  -> CreateEntry           (CreateVolume)
//	Remove -> DeleteEntry           (DeleteVolume)
//	Exists -> LookupDirectoryEntry  (ValidateVolumeCapabilities)
//
// The kubelet-facing side of the driver stays CSI gRPC — that is the Kubernetes
// contract between kubelet and any CSI driver, not a Hanzo choice. This package
// is the only place the driver speaks to storage, and it speaks ZAP.
package filer

import (
	"context"
	"fmt"
	"sync"
	"time"

	filerwire "github.com/hanzoai/s3/s3/wire/filer"
)

// Client is a ZAP connection to one filer endpoint. It is safe for concurrent
// use: the CSI controller serves Create/Delete/Validate concurrently, and the
// underlying transport.Conn is multiplexed per-call by PromiseID, but dial and
// reconnect are serialized here so a broken conn is replaced once, not per call.
type Client struct {
	addr string

	mu   sync.Mutex
	conn *filerwire.Client
}

// New returns a client for addr (e.g. "s3-filer.hanzo.svc.cluster.local:18888").
// It does NOT dial eagerly: the controller starts before the filer may be
// reachable, and a lazy first dial keeps startup order from mattering.
func New(addr string) *Client {
	return &Client{addr: addr}
}

// dial returns a live client, reconnecting if needed.
func (c *Client) dial() (*filerwire.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn, nil
	}
	cl, err := filerwire.Dial("tcp", c.addr)
	if err != nil {
		return nil, fmt.Errorf("filer: dial %s over ZAP: %w", c.addr, err)
	}
	c.conn = cl
	return cl, nil
}

// drop discards a connection believed dead so the next call redials. Passing the
// observed client makes this a no-op if another goroutine already reconnected.
func (c *Client) drop(seen *filerwire.Client) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == seen {
		_ = c.conn.Close()
		c.conn = nil
	}
}

// Close releases the connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
}

// Mkdir creates dir/name as a directory entry. Backs CreateVolume.
//
// Directory mode is 0o777|os.ModeDir expressed as the filer's uint32 mode. The
// entry carries mtime/crtime so a freshly provisioned volume does not appear
// with a zero timestamp to anything that lists it.
func (c *Client) Mkdir(ctx context.Context, dir, name string) error {
	now := time.Now().Unix()
	entry := filerwire.NewEntry(filerwire.EntryInput{
		Name:        name,
		IsDirectory: true,
		Attributes: filerwire.NewFuseAttributes(filerwire.FuseAttributesInput{
			Mtime:    now,
			Crtime:   now,
			FileMode: uint32(0o777) | (1 << 31), // os.ModeDir
		}),
	})
	req := filerwire.NewCreateEntryRequest(filerwire.CreateEntryRequestInput{
		Directory: dir,
		Entry:     entry,
	})
	return c.call(ctx, "mkdir", dir, name, func(cl *filerwire.Client) error {
		resp, err := cl.CreateEntry(ctx, req)
		if err != nil {
			return err
		}
		// CreateEntry reports failure in-band, not as a transport error.
		if e := resp.Error(); e != "" {
			return fmt.Errorf("%s", e)
		}
		return nil
	})
}

// Remove deletes dir/name recursively, including its chunk data. Backs
// DeleteVolume, which must be idempotent — a missing entry is success, since
// CSI may retry a delete whose first attempt already landed.
func (c *Client) Remove(ctx context.Context, dir, name string) error {
	req := filerwire.NewDeleteEntryRequest(filerwire.DeleteEntryRequestInput{
		Directory:            dir,
		Name:                 name,
		IsDeleteData:         true,
		IsRecursive:          true,
		IgnoreRecursiveError: false,
	})
	return c.call(ctx, "remove", dir, name, func(cl *filerwire.Client) error {
		resp, err := cl.DeleteEntry(ctx, req)
		if err != nil {
			return err
		}
		if e := resp.Error(); e != "" {
			return fmt.Errorf("%s", e)
		}
		return nil
	})
}

// Exists reports whether dir/name is present. Backs ValidateVolumeCapabilities.
// A lookup miss is (false, nil) — "absent" is an answer, not a failure — so only
// a transport or filer fault surfaces as an error.
func (c *Client) Exists(ctx context.Context, dir, name string) (bool, error) {
	req := filerwire.NewLookupDirectoryEntryRequest(filerwire.LookupDirectoryEntryRequestInput{
		Directory: dir,
		Name:      name,
	})
	var found bool
	err := c.call(ctx, "exists", dir, name, func(cl *filerwire.Client) error {
		resp, err := cl.LookupDirectoryEntry(ctx, req)
		if err != nil {
			return err
		}
		found = resp.HasEntry()
		return nil
	})
	if err != nil {
		return false, err
	}
	return found, nil
}

// call runs fn against a live connection, redialing once if the first attempt
// fails at the transport. One retry is deliberate: it covers the common case of
// a filer restart having closed an idle pooled connection, without turning a
// genuine filer fault into a retry storm against a struggling backend.
func (c *Client) call(ctx context.Context, op, dir, name string, fn func(*filerwire.Client) error) error {
	cl, err := c.dial()
	if err != nil {
		return err
	}
	if err = fn(cl); err == nil {
		return nil
	}
	c.drop(cl)

	cl2, derr := c.dial()
	if derr != nil {
		return fmt.Errorf("filer %s %s/%s: %v (redial: %w)", op, dir, name, err, derr)
	}
	if err2 := fn(cl2); err2 != nil {
		return fmt.Errorf("filer %s %s/%s: %w", op, dir, name, err2)
	}
	return nil
}
