// Package plugin registers the tarfs snapshotter and discard differ as native containerd plugins.
// Import it blank-side-effect-only alongside containerd's builtins to build a containerd binary with tarfs baked in.
package plugin

import (
	"github.com/values-conflict/containerd-snapshotter-tarfs/snapshotter"

	"github.com/containerd/containerd/v2/core/metadata"
	"github.com/containerd/containerd/v2/plugins"
	cPlugin "github.com/containerd/plugin"
	"github.com/containerd/plugin/registry"
)

func init() {
	// snapshotter: serves OCI layers directly from the content store via FUSE
	registry.Register(&cPlugin.Registration{
		Type:     plugins.SnapshotPlugin,
		ID:       "tarfs",
		Requires: []cPlugin.Type{plugins.MetadataPlugin},
		InitFn: func(ic *cPlugin.InitContext) (any, error) {
			root := ic.Properties[plugins.PropertyRootDir]
			md, err := ic.GetSingle(plugins.MetadataPlugin)
			if err != nil {
				return nil, err
			}
			cs := md.(*metadata.DB).ContentStore()
			return snapshotter.NewSnapshotter(ic.Context, root, cs)
		},
	})

	// differ: fetches the layer blob into the content store but discards all extracted bytes,
	// returning the correct diffID from the decompressed stream; configure with
	// differ = "tarfs" in the unpack_config block alongside snapshotter = "tarfs"
	registry.Register(&cPlugin.Registration{
		Type:     plugins.DiffPlugin,
		ID:       "tarfs",
		Requires: []cPlugin.Type{plugins.MetadataPlugin},
		InitFn: func(ic *cPlugin.InitContext) (any, error) {
			md, err := ic.GetSingle(plugins.MetadataPlugin)
			if err != nil {
				return nil, err
			}
			cs := md.(*metadata.DB).ContentStore()
			return snapshotter.NewDiscardApplier(cs), nil
		},
	})
}
