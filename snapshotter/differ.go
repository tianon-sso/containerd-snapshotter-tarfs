package snapshotter

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/diff"
	"github.com/containerd/containerd/v2/core/mount"
	"github.com/klauspost/compress/zstd"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// DiscardApplier implements [diff.Applier] by computing the diffID of a layer blob via the decompressed stream -- draining all bytes to [io.Discard] rather than extracting them to disk.
// The blob must already be in the content store (the upstream unpack pipeline fetches it before calling Apply).
// The snapshot mounts parameter is intentionally ignored: nothing is written to the snapshot directory.
// This is the native-plugin equivalent of the proxy snapshotter's empty-mounts workaround, except it never writes to disk at all.
type DiscardApplier struct {
	cs content.Store
}

// NewDiscardApplier returns a [DiscardApplier] backed by cs.
func NewDiscardApplier(cs content.Store) *DiscardApplier {
	return &DiscardApplier{cs: cs}
}

// Apply implements [diff.Applier].  It opens the compressed blob from the content store, decompresses it through a SHA-256 digester while draining to [io.Discard], and returns the uncompressed diffID.  The mounts parameter is unused.
func (d *DiscardApplier) Apply(ctx context.Context, desc ocispec.Descriptor, _ []mount.Mount, opts ...diff.ApplyOpt) (ocispec.Descriptor, error) {
	var config diff.ApplyConfig
	for _, o := range opts {
		if err := o(ctx, desc, &config); err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("applying diff opt: %w", err)
		}
	}

	ra, err := d.cs.ReaderAt(ctx, desc)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("opening blob %s: %w", desc.Digest, err)
	}
	defer ra.Close()

	// detect compression by magic bytes -- same detection as openBlobAsFS
	header := make([]byte, 4)
	if _, err := ra.ReadAt(header, 0); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("reading header for %s: %w", desc.Digest, err)
	}

	r := io.NewSectionReader(ra, 0, ra.Size())
	var tarStream io.Reader
	switch {
	case header[0] == 0x1f && header[1] == 0x8b:
		// gzip-compressed layer
		zr, err := gzip.NewReader(r)
		if err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("gzip reader for %s: %w", desc.Digest, err)
		}
		defer zr.Close()
		tarStream = zr
	case header[0] == 0x28 && header[1] == 0xB5 && header[2] == 0x2F && header[3] == 0xFD:
		// zstd-compressed layer (BuildKit default)
		zr, err := zstd.NewReader(r)
		if err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("zstd reader for %s: %w", desc.Digest, err)
		}
		defer zr.Close()
		tarStream = zr
	default:
		// already an uncompressed tar
		tarStream = r
	}

	digester := digest.Canonical.Digester()
	size, err := io.Copy(io.Discard, io.TeeReader(tarStream, digester.Hash()))
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("draining layer stream for %s: %w", desc.Digest, err)
	}

	return ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayer,
		Size:      size,
		Digest:    digester.Digest(),
	}, nil
}
