package main

import (
	"github.com/flanksource/vcluster-sync-all-secrets/syncers"
	"github.com/loft-sh/vcluster-sdk/plugin"
)

func main() {
	ctx := plugin.MustInit()
	plugin.MustRegister(syncers.NewSecretSyncer(ctx))
	plugin.MustStart()
}
