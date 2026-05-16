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

// VersionInfo is immutable build and schema metadata exposed by SystemService.
type VersionInfo struct {
	BinaryVersion string
	GitCommit     string
	BuildDate     string
	SchemaVersion string
}

// SubsystemHealth is one component's contribution to daemon health.
type SubsystemHealth struct {
	Name   string
	OK     bool
	Detail string
}

// MCPConfig contains runtime limits used by MCP tools and resources.
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

// SystemBackend is the daemon-facing dependency set for SystemService.
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

// Area is a configured physical or logical area.
type Area struct {
	ID          string
	DisplayName string
	ParentID    string
}

// Zone groups areas for navigation, policy, and automation targeting.
type Zone struct {
	ID          string
	DisplayName string
	AreaIDs     []string
}

// PageReq is the internal pagination request shared by API adapters.
type PageReq struct {
	Size   uint32
	Cursor Cursor
}

// AreaReader reads configured areas for API handlers.
type AreaReader interface {
	ListAreas(ctx context.Context, page PageReq) ([]Area, Cursor, error)
	GetArea(ctx context.Context, id string) (Area, error)
}

// ZoneReader reads configured zones for API handlers.
type ZoneReader interface {
	ListZones(ctx context.Context, page PageReq) ([]Zone, Cursor, error)
	GetZone(ctx context.Context, id string) (Zone, error)
}

// --- Device ---

// Device is the registry view of a discovered or configured device.
type Device struct {
	ID               string
	FriendlyName     string
	AreaID           string
	DriverInstanceID string
	EntityIDs        []string
}

// DeviceReader reads registry devices for API handlers.
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

// Entity is the API-layer projection of a controllable or observable entity.
type Entity struct {
	ID, Type, DeviceID, AreaID, ZoneID, FriendlyName string
	State                                            *entityv1.Attributes
	Capabilities                                     *entityv1.Attributes
}

// EntitySelector narrows entity reads and subscriptions.
type EntitySelector struct {
	EntityIDs []string
	DeviceIDs []string
	Areas     []string
	Zones     []string
	Classes   []string
}

// EntityReader reads entities from the live registry and state cache.
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

// CapabilityCaller dispatches entity capability calls to the owning driver.
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

// EntityChange is one cursor-addressed update from an entity subscription.
type EntityChange struct {
	EntityID string
	Cursor   uint64
	AtUnixMs int64
	Entity   Entity
}

// --- Driver ---

// Driver describes an available driver implementation.
type Driver struct {
	Name, Version, Description string
	EntityClasses              []string
}

// DriverInstance is the runtime status of one configured driver process.
type DriverInstance struct {
	ID, DriverName, Status string
	EntityCount            uint32
	LastHandshakeUnixMs    int64
}

// DriverControl reads and mutates driver runtime state.
type DriverControl interface {
	ListDrivers(ctx context.Context, page PageReq) ([]Driver, Cursor, error)
	ListInstances(ctx context.Context, page PageReq) ([]DriverInstance, Cursor, error)
	InstanceHealth(ctx context.Context, instanceID string) (ok bool, detail string, err error)
	RestartInstance(ctx context.Context, instanceID, reason, actor string) error
}

// --- Event ---

// Event is the API-layer view of one eventstore row.
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

// EventFilter narrows event queries and tail subscriptions.
type EventFilter struct {
	Kinds        []string
	EntityPrefix string
	Sources      []string
	FromCursor   uint64
	ToCursor     uint64
	FromTime     time.Time
	ToTime       time.Time
}

// EventSource queries and subscribes to the event log.
type EventSource interface {
	Query(ctx context.Context, filter EventFilter, page PageReq) ([]Event, Cursor, error)
	Subscribe(ctx context.Context, filter EventFilter) (<-chan Event, func(), error)
}

// --- Config ---

// ConfigDiff summarizes a config validation or apply delta.
type ConfigDiff struct {
	DriverAdded, DriverRemoved, DriverChanged int32
	EntitiesAdded, EntitiesRemoved            int32
	AutomationsChanged                        int32
	Lines                                     []string
}

// ConfigApplyResult is the outcome of applying a config bundle.
type ConfigApplyResult struct {
	Applied       bool
	Diff          ConfigDiff
	CorrelationID string
	BundleHash    string
	Message       string
	Errors        []string
}

// ConfigChangedEvent is the domain event pushed to subscribers whenever the
// daemon successfully applies a new config bundle.
type ConfigChangedEvent struct {
	AtUnixMs   int64
	BundleHash string
}

// ConfigApplier validates, applies, reloads, and streams config snapshots.
type ConfigApplier interface {
	Validate(ctx context.Context, pklBundle []byte) (valid bool, errs []string, diff ConfigDiff, hash string, err error)
	Apply(ctx context.Context, pklBundle []byte, message, expectedHash string, dryRun, strict bool, actor string) (ConfigApplyResult, error)
	Reload(ctx context.Context, actor string) (diff ConfigDiff, correlationID string, err error)
	CurrentArtifact(ctx context.Context) (*configv1.ConfigSnapshot, error)
	LastReloadError() string
	// SubscribeConfig returns a channel of ConfigChangedEvent and a cancel
	// func that MUST be called to release resources.
	SubscribeConfig() (<-chan ConfigChangedEvent, func())
}

// --- Automation ---

// Automation is the API-layer summary of one configured automation.
type Automation struct {
	ID, DisplayName, Mode string
	Enabled               bool
	InFlight              uint32
	Areas                 []string
}

// TraceEvent is one automation-run trace item.
type TraceEvent struct {
	Cursor       uint64
	At           time.Time
	AutomationID string
	RunID        string
	Kind         string
	Detail       string
	Metadata     map[string]string
}

// AutomationControl reads and mutates automation runtime state.
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

// --- Scene ---

// SceneInvoker is the api-facing seam over scene.Applier.Invoke.
type SceneInvoker interface {
	Invoke(ctx context.Context, sceneID, correlationID, invokedBy string) error
}

// ErrSceneNotFound returns the sentinel handlers translate into
// connect.CodeNotFound. Used by adapters that bridge into SceneInvoker.
func ErrSceneNotFound() error { return errSceneNotFoundSentinel }

// --- Script ---

// Script is the API-layer summary of one invocable Starlark script.
type Script struct {
	Name, Description string
}

// ScriptRunResult is the immediate result of starting or completing a script run.
type ScriptRunResult struct {
	RunID  string
	Result *structpb.Value
}

// StarlarkTestEvent is one streamed test result from a Starlark test file.
type StarlarkTestEvent struct {
	Name, Outcome, Detail string
	At                    time.Time
}

// ScriptRunner executes Starlark scripts and test files for API handlers.
type ScriptRunner interface {
	List(ctx context.Context, page PageReq) ([]Script, Cursor, error)
	Run(ctx context.Context, name string, args map[string]any, actor string) (ScriptRunResult, error)
	Cancel(ctx context.Context, runID string) error
	Eval(ctx context.Context, expr string, actor string) (result *structpb.Value, stdout string, err error)
	RunTests(ctx context.Context, path string) (<-chan StarlarkTestEvent, func(), error)
}
