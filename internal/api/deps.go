package api

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/structpb"

	configv1 "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	entityv1 "github.com/fdatoo/switchyard/gen/switchyard/entity/v1"
	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/auth"
)

type VersionInfo struct {
	BinaryVersion string
	GitCommit     string
	BuildDate     string
	SchemaVersion string
}

type SubsystemHealth struct {
	Name   string
	OK     bool
	Detail string
}

type MCPConfig struct {
	EvalResultMaxBytes       uint32
	ReadFileMaxBytes         uint32
	EntitySubscriptionBuffer uint32
	TraceSubscriptionBuffer  uint32
	TailDefaultWaitSeconds   uint32
	TailMaxWaitSeconds       uint32
}

// EventStoreStats holds size and age information for the event store.
type EventStoreStats struct {
	SizeBytes             uint64
	OldestEventAgeSeconds uint64
	SnapshotCount         uint32
}

type SystemBackend interface {
	Version() VersionInfo
	Health(ctx context.Context) (ok bool, summary string, sub []SubsystemHealth)
	MetricsText() (string, error)
	Diagnostics(ctx context.Context) (bundle []byte, configHash string, generatedAt time.Time, err error)
	// ExportSupportBundle builds and returns a downloadable support bundle.
	ExportSupportBundle(ctx context.Context) (bundle []byte, filename string, configHash string, generatedAt time.Time, err error)
	// EventStoreStats returns size, age, and snapshot count for the event store.
	EventStoreStats(ctx context.Context) (EventStoreStats, error)
	CreateSnapshot(ctx context.Context, owner, reason string) (cursor uint64, createdAt time.Time, err error)
	ConfigDir(ctx context.Context) (string, error)
	MCPConfig(ctx context.Context) (MCPConfig, error)
	RecordConfigFileEdit(ctx context.Context, p auth.Principal, sessionID, path, sha256Hex string, sizeBytes uint32) (uint64, error)
}

// --- Area & Zone ---

type Area struct {
	ID          string
	DisplayName string
	ParentID    string
}

type Zone struct {
	ID          string
	DisplayName string
	AreaIDs     []string
}

type PageReq struct {
	Size   uint32
	Cursor Cursor
}

type AreaReader interface {
	ListAreas(ctx context.Context, page PageReq) ([]Area, Cursor, error)
	GetArea(ctx context.Context, id string) (Area, error)
}

type ZoneReader interface {
	ListZones(ctx context.Context, page PageReq) ([]Zone, Cursor, error)
	GetZone(ctx context.Context, id string) (Zone, error)
}

// --- Device ---

type Device struct {
	ID               string
	FriendlyName     string
	AreaID           string
	DriverInstanceID string
	EntityIDs        []string
}

type DeviceReader interface {
	ListDevices(ctx context.Context, areaID string, page PageReq) ([]Device, Cursor, error)
	GetDevice(ctx context.Context, id string) (Device, error)
}

// DeviceWriter mutates devices and emits the corresponding registry-mutation
// events (DeviceRenamed, DeviceReassigned). actor is the principal id of the
// caller; empty string means "system".
type DeviceWriter interface {
	RenameDevice(ctx context.Context, id, newName, actor string) (Device, error)
	ReassignDevice(ctx context.Context, id, newAreaID, actor string) (Device, error)
}

// --- Entity ---

type Entity struct {
	ID, Type, DeviceID, AreaID, ZoneID, FriendlyName string
	State                                            *entityv1.Attributes
	Capabilities                                     *entityv1.Attributes
}

type EntitySelector struct {
	EntityIDs []string
	DeviceIDs []string
	Areas     []string
	Zones     []string
	Classes   []string
}

type EntityReader interface {
	ListEntities(ctx context.Context, sel EntitySelector, page PageReq) ([]Entity, Cursor, error)
	GetEntity(ctx context.Context, id string) (Entity, error)
}

// CapabilityCallResult is the outcome of dispatching a capability through
// the carport supervisor. CorrelationID is the command id (always set on
// successful dispatch). Success / ErrorMessage carry the driver's
// CommandResult — Success=false with no go-level error means the driver
// rejected the command (bad args, unsupported capability, etc.).
type CapabilityCallResult struct {
	CorrelationID string
	Success       bool
	ErrorMessage  string
}

type CapabilityCaller interface {
	// Call dispatches the capability invocation through the carport supervisor;
	// blocks until the driver acks or ctx is cancelled. The returned error is
	// non-nil only for dispatch-level failures (entity unknown, instance not
	// running, deadline). A driver rejection is reported as Success=false in
	// the result with a non-error return.
	Call(ctx context.Context, entityID, capability string, params map[string]any) (CapabilityCallResult, error)
}

// EntityStreamSource streams live entity changes from the daemon's eventstore.
type EntityStreamSource interface {
	// Subscribe returns a channel of EntityChange events filtered by sel,
	// optionally replaying from fromCursor. The returned cancel func MUST be
	// called to release server-side resources.
	Subscribe(ctx context.Context, sel EntitySelector, fromCursor uint64) (<-chan EntityChange, func(), error)
}

type EntityChange struct {
	EntityID string
	Cursor   uint64
	AtUnixMs int64
	Entity   Entity
}

// --- Driver ---

type Driver struct {
	Name, Version, Description string
	EntityClasses              []string
}

type DriverInstance struct {
	ID, DriverName, Status string
	EntityCount            uint32
	LastHandshakeUnixMs    int64
}

type DriverControl interface {
	ListDrivers(ctx context.Context, page PageReq) ([]Driver, Cursor, error)
	ListInstances(ctx context.Context, page PageReq) ([]DriverInstance, Cursor, error)
	InstanceHealth(ctx context.Context, instanceID string) (ok bool, detail string, err error)
	RestartInstance(ctx context.Context, instanceID, reason, actor string) error
}

// --- Event ---

type Event struct {
	Cursor        uint64
	At            time.Time
	Kind          string
	Entity        string
	Source        string
	CorrelationID string
	CauseID       string
	Payload       *eventv1.Payload
}

type EventFilter struct {
	Kinds        []string
	EntityPrefix string
	Sources      []string
	FromCursor   uint64
	ToCursor     uint64
	FromTime     time.Time
	ToTime       time.Time
}

type EventSource interface {
	Query(ctx context.Context, filter EventFilter, page PageReq) ([]Event, Cursor, error)
	Subscribe(ctx context.Context, filter EventFilter) (<-chan Event, func(), error)
}

// --- Config ---

type ConfigDiff struct {
	DriverAdded, DriverRemoved, DriverChanged int32
	EntitiesAdded, EntitiesRemoved            int32
	AutomationsChanged                        int32
	Lines                                     []string
}

type ConfigApplyResult struct {
	Applied       bool
	Diff          ConfigDiff
	CorrelationID string
	BundleHash    string
	Errors        []string
}

type ConfigApplier interface {
	Validate(ctx context.Context, pklBundle []byte) (valid bool, errs []string, diff ConfigDiff, hash string, err error)
	Apply(ctx context.Context, pklBundle []byte, message, expectedHash string, dryRun, strict bool, actor string) (ConfigApplyResult, error)
	Reload(ctx context.Context, actor string) (diff ConfigDiff, correlationID string, err error)
	CurrentArtifact(ctx context.Context) (*configv1.ConfigSnapshot, error)
}

// --- Automation ---

type Automation struct {
	ID, DisplayName, Mode string
	Enabled               bool
	InFlight              uint32
	Areas                 []string
}

type TraceEvent struct {
	Cursor       uint64
	At           time.Time
	AutomationID string
	RunID        string
	Kind         string
	Detail       string
	Metadata     map[string]string
}

type AutomationControl interface {
	List(ctx context.Context, page PageReq) ([]Automation, Cursor, error)
	Get(ctx context.Context, id string) (Automation, error)
	SetEnabled(ctx context.Context, id string, enabled bool, actor string) (Automation, error)
	Trigger(ctx context.Context, id, actor string) (runID string, err error)
	Trace(ctx context.Context, automationID, runID string, fromCursor uint64) (<-chan TraceEvent, func(), error)
}

// EventAppender appends a raw event payload to the event store.
type EventAppender interface {
	Append(ctx context.Context, payload *eventv1.Payload) (uint64, error)
}

// MCPCapsProvider returns the current MCP capability caps.
type MCPCapsProvider interface {
	MCPConfig(ctx context.Context) (MCPConfig, error)
}

// --- Script ---

type Script struct {
	Name, Description string
}

type ScriptRunResult struct {
	RunID  string
	Result *structpb.Value
}

type StarlarkTestEvent struct {
	Name, Outcome, Detail string
	At                    time.Time
}

type ScriptRunner interface {
	List(ctx context.Context, page PageReq) ([]Script, Cursor, error)
	Run(ctx context.Context, name string, args map[string]any, actor string) (ScriptRunResult, error)
	Cancel(ctx context.Context, runID string) error
	Eval(ctx context.Context, expr string, actor string) (result *structpb.Value, stdout string, err error)
	RunTests(ctx context.Context, path string) (<-chan StarlarkTestEvent, func(), error)
}
