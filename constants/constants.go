package constants

var (
	// PluginName is the name of the plugin
	PluginName = "host-secret-syncer"
	// SyncAnnotation is the annotation that will trigger synchronisation of the secret
	SyncAnnotation = "com.flanksource/vcluster-sync"
	// NamespaceAnnotation is an optional annotation to specify the target namespace
	// within the vcluster
	NamespaceAnnotation = "com.flanksource/vcluster-namespace"
)
