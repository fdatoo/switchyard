// Package registry owns the SQL-backed projection of driver instances,
// devices, entities, and durable event subscriptions.
package registry

import "time"

type DriverInstance struct {
	ID            string
	DriverName    string
	DisplayName   string
	Transport     string
	Endpoint      string
	ConfigHash    string
	Status        string
	LastError     string
	StartedAt     time.Time
	LastHeartbeat time.Time
	CreatedAt     time.Time
}

type Device struct {
	ID               string
	DriverInstanceID string
	FriendlyName     string
	Manufacturer     string
	Model            string
	SwVersion        string
	Metadata         []byte
	Disabled         bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Entity struct {
	ID               string
	DeviceID         string
	DriverInstanceID string
	EntityType       string
	FriendlyName     string
	Capabilities     []byte // serialized entityv1.Attributes
	Disabled         bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type DeviceFilter struct {
	DriverInstanceID string
	IncludeDisabled  bool
}

type EntityFilter struct {
	DriverInstanceID string
	DeviceID         string
	EntityType       string
	IncludeDisabled  bool
}
