package main

import (
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/version"
	bv "github.com/containernetworking/plugins/pkg/utils/buildversion"
	"github.com/royroyee/bvcni/pkg/log"
	"github.com/royroyee/bvcni/plugin"
	"runtime"
)

const (
	pluginName = "bvcni"
	logFile    = "/var/log/bvcni.log"
)

func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

func main() {

	// Init Logger
	log.InitLogger(logFile)

	skel.PluginMain(plugin.CmdAdd, plugin.CmdCheck, plugin.CmdDel, version.All, bv.BuildString(pluginName))
}
