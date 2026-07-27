// containerd-tarfs is a containerd distribution with the tarfs native plugin baked in.
// The tarfs snapshotter and discard differ are registered as built-in plugins; no proxy
// socket or separate process is needed.  Configure with snapshotter = "tarfs" and
// differ = "tarfs" in the unpack_config block.
//
// This binary does NOT import github.com/containerd/containerd/v2/cmd/containerd/builtins,
// so it only has the tarfs plugin registered.  To get a fully functional containerd with
// all standard plugins (overlayfs, native, etc.) plus tarfs, add the builtins blank import
// to a fork of this file alongside this package's deps resolved in a dedicated go.mod:
//
//	_ "github.com/containerd/containerd/v2/cmd/containerd/builtins"
package main

import (
	"fmt"
	"os"

	"github.com/containerd/containerd/v2/cmd/containerd/command"

	_ "github.com/values-conflict/containerd-snapshotter-tarfs/plugin"
)

func main() {
	app := command.App()
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "containerd: %s\n", err)
		os.Exit(1)
	}
}
