package driver

// EntitySpec describes a driver-owned entity.
type EntitySpec struct {
	EntityType   string // "light", "sensor", "switch", etc.
	FriendlyName string
	Capabilities []string // advertised in the manifest
}
