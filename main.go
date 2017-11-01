package main

import (
	"./ggsci"
	"github.com/intelsdi-x/snap-plugin-lib-go/v1/plugin"
)

func main() {
	plugin.StartCollector(ggsci.NewCollector(), ggsci.PluginName, ggsci.PluginVersion)
}
