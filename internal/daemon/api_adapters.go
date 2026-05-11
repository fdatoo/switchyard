package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/common/expfmt"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	entityv1 "github.com/fdatoo/switchyard/gen/switchyard/entity/v1"
	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/api"
	"github.com/fdatoo/switchyard/internal/auth"
	"github.com/fdatoo/switchyard/internal/auth/identity"
	"github.com/fdatoo/switchyard/internal/automation"
	"github.com/fdatoo/switchyard/internal/carport"
	"github.com/fdatoo/switchyard/internal/config"
	"github.com/fdatoo/switchyard/internal/diagnostics"
	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/observability"
	"github.com/fdatoo/switchyard/internal/policy"
	"github.com/fdatoo/switchyard/internal/registry"
	"github.com/fdatoo/switchyard/internal/script"
	ghstarlark "github.com/fdatoo/switchyard/internal/starlark"
	"github.com/fdatoo/switchyard/internal/state"
)

// ---- systemBackendAdapter ----

type systemBackendAdapter struct {
	store   *eventstore.Store
	metrics *observability.Metrics
	phase   *Daemon
}

func (a *systemBackendAdapter) Version() api.VersionInfo {
	return api.VersionInfo{
		BinaryVersion: Version,
		GitCommit:     Commit,
		BuildDate:     "",
		SchemaVersion: "v1alpha1",
	}
}

func (a *systemBackendAdapter) Health(_ context.Context) (bool, string, []api.SubsystemHealth) {
	status, code := a.phase.healthStatus()
	ok := code == 200
	return ok, status, nil
}

func (a *systemBackendAdapter) MetricsText() (string, error) {
	if a.metrics == nil || a.metrics.Registry == nil {
		return "", nil
	}
	mfs, err := a.metrics.Registry.Gather()
	if err != nil {
		// Some metrics may still be valid; log but continue
		if len(mfs) == 0 {
			return "", fmt.Errorf("gather metrics: %w", err)
		}
	}
	var buf bytes.Buffer
	enc := expfmt.NewEncoder(&buf, expfmt.NewFormat(expfmt.TypeTextPlain))
	for _, mf := range mfs {
		if encErr := enc.Encode(mf); encErr != nil {
			return "", fmt.Errorf("encode metric: %w", encErr)
		}
	}
	return buf.String(), nil
}

func (a *systemBackendAdapter) Diagnostics(ctx context.Context) ([]byte, string, time.Time, error) {
	generatedAt := time.Now()

	metricsDump, err := a.MetricsText()
	if err != nil {
		return nil, "", time.Time{}, err
	}

	var events []eventstore.Event
	if a.store != nil {
		latest := a.store.LatestPosition()
		var from uint64
		if latest > 1000 {
			from = latest - 1000
		}
		events, err = a.store.Query(ctx, eventstore.QueryOptions{FromPosition: from, Limit: 1000})
		if err != nil {
			return nil, "", time.Time{}, fmt.Errorf("events tail: %w", err)
		}
	}

	var cursors []observability.ProjectionCursor
	var snap *configpb.ConfigSnapshot
	health := diagnostics.HealthInfo{Status: "unknown"}
	if a.phase != nil {
		status, _ := a.phase.healthStatus()
		health.Status = status
		health.Phase = a.phase.phase.Load()
		health.InRecovery = a.phase.InRecovery()
		if !a.phase.startTime.IsZero() {
			health.UptimeSeconds = generatedAt.Sub(a.phase.startTime).Seconds()
		}
		if health.InRecovery {
			reason, failedPos := a.phase.RecoveryInfo()
			health.Recovery = &diagnostics.RecoveryInfo{Reason: reason, FailedPosition: failedPos}
		}
		if a.phase.db != nil {
			cursors, err = a.phase.QueryProjectionCursors(ctx)
			if err != nil {
				return nil, "", time.Time{}, fmt.Errorf("projection cursors: %w", err)
			}
		}
		if a.phase.configMgr != nil {
			snap = a.phase.configMgr.CurrentRedacted()
		}
	}

	return diagnostics.Build(diagnostics.Options{
		BuildInfo: diagnostics.BuildInfo{
			Version:   Version,
			Commit:    Commit,
			GoVersion: GoVersion,
		},
		MetricsDump:       metricsDump,
		EventsTail:        events,
		ProjectionCursors: cursors,
		ConfigSnapshot:    snap,
		Health:            health,
		GeneratedAt:       generatedAt,
	})
}

func (a *systemBackendAdapter) CreateSnapshot(ctx context.Context, _, _ string) (uint64, time.Time, error) {
	if a.store == nil {
		return 0, time.Time{}, fmt.Errorf("store not available")
	}
	pos, err := a.store.SnapshotNow(ctx, "")
	if err != nil {
		return 0, time.Time{}, err
	}
	return pos, time.Now(), nil
}

func (a *systemBackendAdapter) ConfigDir(_ context.Context) (string, error) {
	return a.phase.configDir, nil
}

func (a *systemBackendAdapter) MCPConfig(_ context.Context) (api.MCPConfig, error) {
	defaults := api.MCPConfig{
		EvalResultMaxBytes:       65536,
		ReadFileMaxBytes:         1048576,
		EntitySubscriptionBuffer: 256,
		TraceSubscriptionBuffer:  1024,
		TailDefaultWaitSeconds:   0,
		TailMaxWaitSeconds:       60,
	}
	if a.phase.configMgr == nil {
		return defaults, nil
	}
	snap := a.phase.configMgr.Current()
	if snap == nil || snap.GetMcp() == nil {
		return defaults, nil
	}
	mcp := snap.GetMcp()
	cfg := defaults
	if v := mcp.GetEvalResultMaxBytes(); v != 0 {
		cfg.EvalResultMaxBytes = v
	}
	if v := mcp.GetReadFileMaxBytes(); v != 0 {
		cfg.ReadFileMaxBytes = v
	}
	if v := mcp.GetEntitySubscriptionBuffer(); v != 0 {
		cfg.EntitySubscriptionBuffer = v
	}
	if v := mcp.GetTraceSubscriptionBuffer(); v != 0 {
		cfg.TraceSubscriptionBuffer = v
	}
	cfg.TailDefaultWaitSeconds = mcp.GetTailDefaultWaitSeconds()
	if v := mcp.GetTailMaxWaitSeconds(); v != 0 {
		cfg.TailMaxWaitSeconds = v
	}
	return cfg, nil
}

// ExportSupportBundle builds a full diagnostics bundle and returns it as the
// downloadable support archive. Delegates to Diagnostics for the bundle bytes
// and adds a structured filename. Added by UI v2 plan 09.
func (a *systemBackendAdapter) ExportSupportBundle(ctx context.Context) ([]byte, string, string, time.Time, error) {
	bundle, configHash, generatedAt, err := a.Diagnostics(ctx)
	if err != nil {
		return nil, "", "", time.Time{}, err
	}
	filename := fmt.Sprintf("switchyard-support-%s.zip", generatedAt.UTC().Format("20060102-150405"))
	return bundle, filename, configHash, generatedAt, nil
}

// EventStoreStats returns size, age, and snapshot count for the event store.
// Added by UI v2 plan 09.
// TODO: expose store.SizeBytes(), store.OldestEventAge(), and store.SnapshotCount()
// from eventstore.Store once those methods are added.
func (a *systemBackendAdapter) EventStoreStats(_ context.Context) (api.EventStoreStats, error) {
	// Return zero values until the eventstore exposes these metrics.
	return api.EventStoreStats{}, nil
}

func (a *systemBackendAdapter) RecordConfigFileEdit(ctx context.Context, p auth.Principal, sessionID, path, sha256Hex string, sizeBytes uint32) (uint64, error) {
	cfgDir := a.phase.configDir
	if cfgDir == "" {
		return 0, fmt.Errorf("config dir not available")
	}
	abs := filepath.Join(cfgDir, path)
	rel, err := filepath.Rel(cfgDir, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return 0, api.ErrPathEscape
	}
	pos, err := a.store.Append(ctx, eventstore.Event{
		Kind:      "config_file_edited",
		Source:    "api:" + p.ID,
		Timestamp: time.Now(),
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_ConfigFileEdited{
			ConfigFileEdited: &eventv1.ConfigFileEdited{
				PrincipalId: p.ID,
				SessionId:   sessionID,
				Path:        path,
				Sha256Hex:   sha256Hex,
				SizeBytes:   sizeBytes,
			},
		}},
	})
	if err != nil {
		return 0, err
	}
	return pos, nil
}

// ---- areaReaderAdapter ----

type areaReaderAdapter struct {
	reg *registry.Registry
}

func (a *areaReaderAdapter) ListAreas(_ context.Context, _ api.PageReq) ([]api.Area, api.Cursor, error) {
	// Areas are not tracked in the registry yet (future milestone)
	return nil, api.Cursor{}, nil
}

func (a *areaReaderAdapter) GetArea(_ context.Context, id string) (api.Area, error) {
	return api.Area{}, fmt.Errorf("area %q not found", id)
}

// ---- zoneReaderAdapter ----

type zoneReaderAdapter struct {
	reg *registry.Registry
}

func (a *zoneReaderAdapter) ListZones(_ context.Context, _ api.PageReq) ([]api.Zone, api.Cursor, error) {
	// Zones are not tracked in the registry yet (future milestone)
	return nil, api.Cursor{}, nil
}

func (a *zoneReaderAdapter) GetZone(_ context.Context, id string) (api.Zone, error) {
	return api.Zone{}, fmt.Errorf("zone %q not found", id)
}

// ---- deviceReaderAdapter ----

type deviceReaderAdapter struct {
	reg *registry.Registry
}

func (a *deviceReaderAdapter) ListDevices(ctx context.Context, areaID string, page api.PageReq) ([]api.Device, api.Cursor, error) {
	devs, err := a.reg.ListDevices(ctx, registry.DeviceFilter{})
	if err != nil {
		return nil, api.Cursor{}, err
	}
	// Filter by areaID if provided — registry devices don't have areaID yet, skip filter
	out := make([]api.Device, 0, len(devs))
	for _, d := range devs {
		out = append(out, api.Device{
			ID:               d.ID,
			FriendlyName:     d.FriendlyName,
			DriverInstanceID: d.DriverInstanceID,
		})
	}
	// Simple in-memory pagination
	return paginateDevices(out, page)
}

func (a *deviceReaderAdapter) GetDevice(ctx context.Context, id string) (api.Device, error) {
	d, err := a.reg.GetDevice(ctx, id)
	if err != nil {
		return api.Device{}, err
	}
	return api.Device{
		ID:               d.ID,
		FriendlyName:     d.FriendlyName,
		DriverInstanceID: d.DriverInstanceID,
	}, nil
}

// ---- deviceWriterAdapter ----

type deviceWriterAdapter struct {
	reg   *registry.Registry
	store *eventstore.Store
}

func (a *deviceWriterAdapter) RenameDevice(ctx context.Context, id, newName, actor string) (api.Device, error) {
	// Append a DeviceRenamed event; the registry projector picks it up
	_, err := a.store.Append(ctx, eventstore.Event{
		Kind:      "device_renamed",
		Source:    "api:" + actor,
		Timestamp: time.Now(),
		Entity:    id,
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_DeviceRenamed{
			DeviceRenamed: &eventv1.DeviceRenamed{
				DeviceId:        id,
				NewFriendlyName: newName,
			},
		}},
	})
	if err != nil {
		return api.Device{}, err
	}
	d, err := a.reg.GetDevice(ctx, id)
	if err != nil {
		return api.Device{}, err
	}
	return api.Device{
		ID:               d.ID,
		FriendlyName:     d.FriendlyName,
		DriverInstanceID: d.DriverInstanceID,
	}, nil
}

func (a *deviceWriterAdapter) ReassignDevice(ctx context.Context, id, newAreaID, actor string) (api.Device, error) {
	// Append a DeviceReassigned event
	_, err := a.store.Append(ctx, eventstore.Event{
		Kind:      "device_reassigned",
		Source:    "api:" + actor,
		Timestamp: time.Now(),
		Entity:    id,
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_DeviceReassigned{
			DeviceReassigned: &eventv1.DeviceReassigned{
				DeviceId:  id,
				NewAreaId: newAreaID,
			},
		}},
	})
	if err != nil {
		return api.Device{}, err
	}
	d, err := a.reg.GetDevice(ctx, id)
	if err != nil {
		return api.Device{}, err
	}
	return api.Device{
		ID:               d.ID,
		FriendlyName:     d.FriendlyName,
		DriverInstanceID: d.DriverInstanceID,
		AreaID:           newAreaID,
	}, nil
}

// ---- entityReaderAdapter ----

type entityReaderAdapter struct {
	reg   *registry.Registry
	cache *state.Cache
}

func (a *entityReaderAdapter) ListEntities(ctx context.Context, sel api.EntitySelector, page api.PageReq) ([]api.Entity, api.Cursor, error) {
	filter := registry.EntityFilter{}
	if len(sel.DeviceIDs) == 1 {
		filter.DeviceID = sel.DeviceIDs[0]
	}
	if len(sel.Classes) == 1 {
		filter.EntityType = sel.Classes[0]
	}
	ents, err := a.reg.ListEntities(ctx, filter)
	if err != nil {
		return nil, api.Cursor{}, err
	}
	out := make([]api.Entity, 0, len(ents))
	for _, e := range ents {
		ent := a.toAPIEntity(e)
		if !entityMatchesSelector(ent, sel) {
			continue
		}
		out = append(out, ent)
	}
	return paginateEntities(out, page)
}

func (a *entityReaderAdapter) GetEntity(ctx context.Context, id string) (api.Entity, error) {
	e, err := a.reg.GetEntity(ctx, id)
	if err != nil {
		return api.Entity{}, err
	}
	return a.toAPIEntity(e), nil
}

func (a *entityReaderAdapter) toAPIEntity(e registry.Entity) api.Entity {
	ent := api.Entity{
		ID:           e.ID,
		Type:         e.EntityType,
		DeviceID:     e.DeviceID,
		FriendlyName: e.FriendlyName,
	}
	// Attach live state from cache if available
	if a.cache != nil {
		if st, ok := a.cache.Get(e.ID); ok {
			ent.State = st.Attributes
		}
	}
	return ent
}

// ---- entityStreamSourceAdapter ----

type entityStreamSourceAdapter struct {
	store  *eventstore.Store
	reader api.EntityReader
}

func (a *entityStreamSourceAdapter) Subscribe(ctx context.Context, sel api.EntitySelector, fromCursor uint64) (<-chan api.EntityChange, func(), error) {
	if a.store == nil {
		return nil, nil, fmt.Errorf("eventstore not available for entity stream")
	}
	if a.reader == nil {
		return nil, nil, fmt.Errorf("entity reader not available for entity stream")
	}

	subCtx, cancel := context.WithCancel(ctx)
	sub, err := a.store.Subscribe(subCtx, eventstore.SubscribeOptions{
		FromPosition: fromCursor,
		Filter: eventstore.Filter{
			Kinds:    []string{"state_changed"},
			Entities: sel.EntityIDs,
		},
	})
	if err != nil {
		cancel()
		return nil, nil, err
	}

	ch := make(chan api.EntityChange, 64)
	go func() {
		defer close(ch)
		defer func() { _ = sub.Close() }()
		for {
			select {
			case ev, ok := <-sub.C():
				if !ok {
					return
				}
				change, ok := a.entityChangeFromEvent(subCtx, sel, ev)
				if !ok {
					continue
				}
				select {
				case ch <- change:
				case <-subCtx.Done():
					return
				}
			case <-subCtx.Done():
				return
			}
		}
	}()

	return ch, cancel, nil
}

func (a *entityStreamSourceAdapter) entityChangeFromEvent(ctx context.Context, sel api.EntitySelector, ev eventstore.Event) (api.EntityChange, bool) {
	payload := ev.Payload.GetStateChanged()
	if payload == nil {
		return api.EntityChange{}, false
	}
	entityID := ev.Entity
	if entityID == "" {
		entityID = payload.GetEntityId()
	}
	if entityID == "" {
		return api.EntityChange{}, false
	}

	ent, err := a.reader.GetEntity(ctx, entityID)
	if err != nil {
		return api.EntityChange{}, false
	}
	if payload.Attributes != nil {
		ent.State = proto.Clone(payload.Attributes).(*entityv1.Attributes)
	}
	if !entityMatchesSelector(ent, sel) {
		return api.EntityChange{}, false
	}

	return api.EntityChange{
		EntityID: entityID,
		Cursor:   ev.Position,
		AtUnixMs: ev.Timestamp.UnixMilli(),
		Entity:   ent,
	}, true
}

func entityMatchesSelector(e api.Entity, sel api.EntitySelector) bool {
	return stringInSelector(sel.EntityIDs, e.ID) &&
		stringInSelector(sel.DeviceIDs, e.DeviceID) &&
		stringInSelector(sel.Areas, e.AreaID) &&
		stringInSelector(sel.Zones, e.ZoneID) &&
		stringInSelector(sel.Classes, e.Type)
}

func stringInSelector(values []string, got string) bool {
	if len(values) == 0 {
		return true
	}
	for _, v := range values {
		if v == got {
			return true
		}
	}
	return false
}

// ---- capabilityCallerAdapter ----

type capabilityCallerAdapter struct {
	sup *carport.Host
}

func (a *capabilityCallerAdapter) Call(ctx context.Context, entityID, capability string, params map[string]any) (api.CapabilityCallResult, error) {
	// Convert map[string]any to map[string]string
	strParams := make(map[string]string, len(params))
	for k, v := range params {
		strParams[k] = fmt.Sprintf("%v", v)
	}
	result, err := a.sup.Dispatch(ctx, entityID, capability, strParams)
	if err != nil {
		return api.CapabilityCallResult{}, err
	}
	out := api.CapabilityCallResult{Success: result.GetOk(), ErrorMessage: result.GetErrorMessage()}
	if cid := result.GetCommandId(); cid != "" {
		out.CorrelationID = cid
	}
	return out, nil
}

// ---- driverControlAdapter ----

type driverControlAdapter struct {
	sup *carport.Host
	reg *registry.Registry
}

func (a *driverControlAdapter) ListDrivers(_ context.Context, _ api.PageReq) ([]api.Driver, api.Cursor, error) {
	// Driver metadata is not yet exposed from carport; return empty list
	return nil, api.Cursor{}, nil
}

func (a *driverControlAdapter) ListInstances(ctx context.Context, page api.PageReq) ([]api.DriverInstance, api.Cursor, error) {
	instances, err := a.reg.ListDriverInstances(ctx)
	if err != nil {
		return nil, api.Cursor{}, err
	}
	out := make([]api.DriverInstance, 0, len(instances))
	for _, di := range instances {
		var lastHB int64
		if !di.LastHeartbeat.IsZero() {
			lastHB = di.LastHeartbeat.UnixMilli()
		}
		out = append(out, api.DriverInstance{
			ID:                  di.ID,
			DriverName:          di.DriverName,
			Status:              di.Status,
			LastHandshakeUnixMs: lastHB,
		})
	}
	return paginateDriverInstances(out, page)
}

func (a *driverControlAdapter) InstanceHealth(_ context.Context, instanceID string) (bool, string, error) {
	if a.sup == nil {
		return false, "supervisor not available", nil
	}
	state := a.sup.InstanceState(instanceID)
	ok := state == carport.StateRunning
	return ok, state.String(), nil
}

func (a *driverControlAdapter) RestartInstance(ctx context.Context, instanceID, _, _ string) error {
	if a.sup == nil {
		return fmt.Errorf("supervisor not available")
	}
	return a.sup.RestartInstance(ctx, instanceID)
}

// ---- eventSourceAdapter ----

type eventSourceAdapter struct {
	store *eventstore.Store
}

func (a *eventSourceAdapter) Query(ctx context.Context, filter api.EventFilter, page api.PageReq) ([]api.Event, api.Cursor, error) {
	opts := eventstore.QueryOptions{
		FromPosition: filter.FromCursor,
		ToPosition:   filter.ToCursor,
		Filter: eventstore.Filter{
			Kinds:   filter.Kinds,
			Sources: filter.Sources,
		},
		Limit: int(page.Size),
	}
	events, err := a.store.Query(ctx, opts)
	if err != nil {
		return nil, api.Cursor{}, err
	}
	out := make([]api.Event, 0, len(events))
	var lastPos uint64
	for _, e := range events {
		out = append(out, api.Event{
			Cursor:        e.Position,
			At:            e.Timestamp,
			Kind:          e.Kind,
			Entity:        e.Entity,
			Source:        e.Source,
			CorrelationID: e.CorrelationID.String(),
			Payload:       e.Payload,
		})
		lastPos = e.Position
	}
	var next api.Cursor
	if uint32(len(events)) >= page.Size && page.Size > 0 {
		next = api.Cursor{Position: lastPos}
	}
	return out, next, nil
}

func (a *eventSourceAdapter) Subscribe(ctx context.Context, filter api.EventFilter) (<-chan api.Event, func(), error) {
	sub, err := a.store.Subscribe(ctx, eventstore.SubscribeOptions{
		FromPosition: filter.FromCursor,
		Filter: eventstore.Filter{
			Kinds:   filter.Kinds,
			Sources: filter.Sources,
		},
	})
	if err != nil {
		return nil, nil, err
	}

	ch := make(chan api.Event, 64)
	go func() {
		defer close(ch)
		defer func() { _ = sub.Close() }()
		for {
			select {
			case e, ok := <-sub.C():
				if !ok {
					return
				}
				select {
				case ch <- api.Event{
					Cursor:        e.Position,
					At:            e.Timestamp,
					Kind:          e.Kind,
					Entity:        e.Entity,
					Source:        e.Source,
					CorrelationID: e.CorrelationID.String(),
					Payload:       e.Payload,
				}:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	cancel := func() { _ = sub.Close() }
	return ch, cancel, nil
}

// ---- configApplierAdapter ----

type configApplierAdapter struct {
	mgr *config.Manager
}

func (a *configApplierAdapter) Validate(ctx context.Context, _ []byte) (bool, []string, api.ConfigDiff, string, error) {
	if a.mgr == nil {
		return false, []string{"config manager not available"}, api.ConfigDiff{}, "", nil
	}
	_, diff, err := a.mgr.Validate(ctx)
	if err != nil {
		return false, []string{err.Error()}, api.ConfigDiff{}, "", nil
	}
	d := api.ConfigDiff{}
	if diff != nil {
		d.DriverAdded = int32(len(diff.DriverInstancesAdded))
		d.DriverRemoved = int32(len(diff.DriverInstancesRemoved))
		d.DriverChanged = int32(len(diff.DriverInstancesChanged))
	}
	return true, nil, d, "", nil
}

func (a *configApplierAdapter) Apply(ctx context.Context, _ []byte, _, _ string, dryRun, _ bool, _ string) (api.ConfigApplyResult, error) {
	if a.mgr == nil {
		return api.ConfigApplyResult{}, fmt.Errorf("config manager not available")
	}
	if err := a.mgr.Apply(ctx, dryRun); err != nil {
		return api.ConfigApplyResult{Errors: []string{err.Error()}}, nil
	}
	return api.ConfigApplyResult{Applied: !dryRun}, nil
}

func (a *configApplierAdapter) Reload(ctx context.Context, _ string) (api.ConfigDiff, string, error) {
	if a.mgr == nil {
		return api.ConfigDiff{}, "", fmt.Errorf("config manager not available")
	}
	if err := a.mgr.Apply(ctx, false); err != nil {
		return api.ConfigDiff{}, "", err
	}
	return api.ConfigDiff{}, "", nil
}

func (a *configApplierAdapter) CurrentArtifact(_ context.Context) (*configpb.ConfigSnapshot, error) {
	if a.mgr == nil {
		return &configpb.ConfigSnapshot{}, nil
	}
	snap := a.mgr.Current()
	if snap == nil {
		return &configpb.ConfigSnapshot{}, nil
	}
	return snap, nil
}

// ---- automationControlAdapter ----

type automationControlAdapter struct {
	eng   *automation.Engine
	store *eventstore.Store
}

func (a *automationControlAdapter) List(_ context.Context, page api.PageReq) ([]api.Automation, api.Cursor, error) {
	if a.eng == nil {
		return nil, api.Cursor{}, nil
	}
	summaries := a.eng.List()
	out := make([]api.Automation, 0, len(summaries))
	for _, s := range summaries {
		out = append(out, api.Automation{
			ID:      s.ID,
			Mode:    s.Mode,
			Enabled: s.Enabled,
		})
	}
	return paginateAutomations(out, page)
}

func (a *automationControlAdapter) Get(_ context.Context, id string) (api.Automation, error) {
	if a.eng == nil {
		return api.Automation{}, fmt.Errorf("automation %q not found", id)
	}
	s, ok := a.eng.Get(id)
	if !ok {
		return api.Automation{}, fmt.Errorf("automation %q not found", id)
	}
	return api.Automation{
		ID:      s.ID,
		Mode:    s.Mode,
		Enabled: s.Enabled,
	}, nil
}

func (a *automationControlAdapter) SetEnabled(_ context.Context, id string, enabled bool, _ string) (api.Automation, error) {
	if a.eng == nil {
		return api.Automation{}, fmt.Errorf("automation engine not available")
	}
	if err := a.eng.SetEnabled(id, enabled); err != nil {
		return api.Automation{}, err
	}
	s, ok := a.eng.Get(id)
	if !ok {
		return api.Automation{}, fmt.Errorf("automation %q not found after update", id)
	}
	return api.Automation{
		ID:      s.ID,
		Mode:    s.Mode,
		Enabled: s.Enabled,
	}, nil
}

func (a *automationControlAdapter) Trigger(ctx context.Context, id, actor string) (string, error) {
	if a.eng == nil {
		return "", fmt.Errorf("automation engine not available")
	}
	if err := a.eng.Trigger(ctx, id, actor); err != nil {
		return "", err
	}
	return "", nil
}

func (a *automationControlAdapter) Trace(ctx context.Context, automationID, runID string, fromCursor uint64) (<-chan api.TraceEvent, func(), error) {
	if a.store == nil {
		return nil, nil, fmt.Errorf("eventstore not available for trace")
	}
	filter := eventstore.Filter{
		Kinds:   []string{"automation_triggered", "automation_finished"},
		Sources: []string{"automation:" + automationID},
	}
	if runID != "" {
		uid, err := uuid.Parse(runID)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid run_id %q: %w", runID, err)
		}
		filter.CorrelationIDs = []uuid.UUID{uid}
	}
	subCtx, cancel := context.WithCancel(ctx)
	sub, err := a.store.Subscribe(subCtx, eventstore.SubscribeOptions{
		FromPosition: fromCursor,
		Filter:       filter,
	})
	if err != nil {
		cancel()
		return nil, nil, err
	}
	ch := make(chan api.TraceEvent, 32)
	go func() {
		defer close(ch)
		defer func() { _ = sub.Close() }()
		for ev := range sub.C() {
			te := traceEventFromStoreEvent(ev)
			select {
			case ch <- te:
			case <-subCtx.Done():
				return
			}
		}
	}()
	return ch, cancel, nil
}

func traceEventFromStoreEvent(ev eventstore.Event) api.TraceEvent {
	te := api.TraceEvent{
		Cursor: ev.Position,
		At:     ev.Timestamp,
		Kind:   ev.Kind,
		RunID:  ev.CorrelationID.String(),
	}
	if after, ok := strings.CutPrefix(ev.Source, "automation:"); ok {
		te.AutomationID = after
	}
	meta := map[string]string{}
	if ev.Payload != nil {
		switch p := ev.Payload.Kind.(type) {
		case *eventv1.Payload_AutomationTriggered:
			at := p.AutomationTriggered
			if at.AutomationId != "" {
				te.AutomationID = at.AutomationId
			}
			te.Detail = at.TriggerKind
			if at.InvokedBy != "" {
				meta["invoked_by"] = at.InvokedBy
			}
			if at.TriggerEventPosition != 0 {
				meta["trigger_event_position"] = strconv.FormatInt(int64(at.TriggerEventPosition), 10)
			}
		case *eventv1.Payload_AutomationFinished:
			af := p.AutomationFinished
			if af.AutomationId != "" {
				te.AutomationID = af.AutomationId
			}
			te.Detail = af.Outcome.String()
			meta["elapsed_ms"] = strconv.FormatInt(af.ElapsedMs, 10)
			meta["starlark_steps"] = strconv.FormatUint(af.StarlarkSteps, 10)
			if af.Error != "" {
				meta["error"] = af.Error
			}
		}
	}
	if len(meta) > 0 {
		te.Metadata = meta
	}
	return te
}

// ---- scriptRunnerAdapter ----

type scriptRunnerAdapter struct {
	eng       *script.Engine
	rt        *ghstarlark.Runtime
	configDir string
}

func (a *scriptRunnerAdapter) List(_ context.Context, page api.PageReq) ([]api.Script, api.Cursor, error) {
	if a.eng == nil {
		return nil, api.Cursor{}, nil
	}
	names := a.eng.List()
	out := make([]api.Script, 0, len(names))
	for _, name := range names {
		out = append(out, api.Script{Name: name})
	}
	return paginateScripts(out, page)
}

func (a *scriptRunnerAdapter) Run(ctx context.Context, name string, args map[string]any, actor string) (api.ScriptRunResult, error) {
	if a.eng == nil {
		return api.ScriptRunResult{}, fmt.Errorf("script engine not available")
	}
	// Convert map[string]any to map[string]string
	strArgs := make(map[string]string, len(args))
	for k, v := range args {
		strArgs[k] = fmt.Sprintf("%v", v)
	}
	res, err := a.eng.Call(ctx, name, strArgs, actor, "")
	if err != nil {
		return api.ScriptRunResult{}, err
	}
	return api.ScriptRunResult{
		RunID: res.CorrelationID,
	}, nil
}

func (a *scriptRunnerAdapter) Cancel(ctx context.Context, runID string) error {
	if a.eng == nil {
		return fmt.Errorf("script engine not available")
	}
	canceledBy := "unknown"
	if p, ok := auth.PrincipalFromContext(ctx); ok && p.ID != "" {
		canceledBy = p.ID
	}
	if err := a.eng.Cancel(ctx, runID, canceledBy); err != nil {
		if errors.Is(err, script.ErrRunNotFound) {
			return api.ErrRunNotFound
		}
		return err
	}
	return nil
}

func (a *scriptRunnerAdapter) Eval(ctx context.Context, expr, _ string) (*structpb.Value, string, error) {
	if a.rt == nil {
		return nil, "", fmt.Errorf("starlark runtime not available")
	}
	res, err := a.rt.Execute(ctx, ghstarlark.KindMCPEval, expr, nil)
	if err != nil {
		return nil, "", err
	}
	var stdout string
	if res != nil {
		stdout = strings.Join(res.Logs, "\n")
	}
	// Return nil result for now — structpb conversion requires value inspection
	return nil, stdout, nil
}

var testFnRegex = regexp.MustCompile(`(?m)^def (test_\w+)\(`)

func (a *scriptRunnerAdapter) RunTests(ctx context.Context, path string) (<-chan api.StarlarkTestEvent, func(), error) {
	if a.rt == nil {
		return nil, nil, fmt.Errorf("starlark runtime not available")
	}
	if path == "" {
		return nil, nil, fmt.Errorf("path is required")
	}
	content, err := os.ReadFile(path)
	if err != nil && a.configDir != "" && !filepath.IsAbs(path) {
		content, err = os.ReadFile(filepath.Join(a.configDir, path))
	}
	if err != nil {
		return nil, nil, err
	}
	src := string(content)

	matches := testFnRegex.FindAllStringSubmatch(src, -1)
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, m[1])
	}
	sort.Strings(names)

	ch := make(chan api.StarlarkTestEvent, len(names)+1)
	go func() {
		defer close(ch)
		if len(names) == 0 {
			// No test_ functions found; run once to surface parse errors.
			if _, execErr := a.rt.ExecuteTest(ctx, src, "__probe__"); execErr != nil &&
				!strings.Contains(execErr.Error(), "not found") {
				ch <- api.StarlarkTestEvent{Name: path, Outcome: "fail", Detail: execErr.Error(), At: time.Now()}
			}
			return
		}
		for _, name := range names {
			start := time.Now()
			_, execErr := a.rt.ExecuteTest(ctx, src, name)
			ev := api.StarlarkTestEvent{Name: name, At: start}
			if execErr != nil {
				ev.Outcome = "fail"
				ev.Detail = execErr.Error()
			} else {
				ev.Outcome = "ok"
			}
			ch <- ev
		}
	}()
	return ch, func() {}, nil
}

// ---- eventAppenderAdapter ----

type eventAppenderAdapter struct{ store *eventstore.Store }

func (a *eventAppenderAdapter) Append(ctx context.Context, payload *eventv1.Payload) (uint64, error) {
	ev := eventstore.Event{
		Kind:      "mcp_eval_requested",
		Source:    "api:mcp",
		Timestamp: time.Now(),
		Payload:   payload,
	}
	return a.store.Append(ctx, ev)
}

// ---- webhookRouterAdapter ----

type webhookRouterAdapter struct {
	mgr *config.Manager
}

func (a *webhookRouterAdapter) SecretFor(slug string) (string, bool) {
	if a.mgr == nil {
		return "", false
	}
	snap := a.mgr.Current()
	if snap == nil {
		return "", false
	}
	// Find a webhook trigger with matching path
	for _, auto := range snap.GetAutomations() {
		for _, trig := range auto.GetTriggers() {
			if wh := trig.GetWebhook(); wh != nil && wh.GetPath() == slug {
				// Webhook secrets are not in the proto snapshot yet (future milestone)
				// Return empty secret, webhook still accepted (verifySignature will fail unless empty sig sent)
				return "", true
			}
		}
	}
	return "", false
}

func (a *webhookRouterAdapter) MaxBodyBytes() int64 {
	if a.mgr == nil {
		return 1 << 20 // 1 MiB default
	}
	snap := a.mgr.Current()
	if snap == nil {
		return 1 << 20
	}
	if lc := snap.GetListener(); lc != nil {
		if wc := lc.GetWebhooks(); wc != nil {
			if mb := wc.GetMaxBodyBytes(); mb > 0 {
				return mb
			}
		}
	}
	return 1 << 20
}

// ---- webhookAppenderAdapter ----

type webhookAppenderAdapter struct {
	store *eventstore.Store
}

func (a *webhookAppenderAdapter) AppendWebhook(ctx context.Context, w api.AppendedWebhook) error {
	_, err := a.store.Append(ctx, eventstore.Event{
		Kind:      "webhook_received",
		Source:    "webhook:" + w.Slug,
		Timestamp: time.Now(),
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_WebhookReceived{
			WebhookReceived: &eventv1.WebhookReceived{
				Slug:     w.Slug,
				Body:     w.Body,
				Headers:  w.Headers,
				SourceIp: w.SourceIP,
			},
		}},
	})
	return err
}

// ---- webhookMetricsAdapter ----

type webhookMetricsAdapter struct {
	m *observability.Metrics
}

func (a *webhookMetricsAdapter) Inc(slug, result string) {
	if a.m != nil && a.m.APIWebhookReceivedTotal != nil {
		a.m.APIWebhookReceivedTotal.WithLabelValues(slug, result).Inc()
	}
}

// ---- roleAdapter ----

// roleAdapter implements policy.Roles using the identity store.
type roleAdapter struct{ store *identity.Store }

func (ra roleAdapter) For(p auth.Principal) map[policy.RoleSlug]struct{} {
	slug := strings.TrimPrefix(p.ID, "user:")
	roles, err := ra.store.RolesFor(context.Background(), slug)
	if err != nil || len(roles) == 0 {
		return nil
	}
	m := make(map[policy.RoleSlug]struct{}, len(roles))
	for _, r := range roles {
		m[policy.RoleSlug(r)] = struct{}{}
	}
	return m
}

// ---- compilePolicyFromSnapshot ----

func compilePolicyFromSnapshot(snap *configpb.ConfigSnapshot) (*policy.Compiled, error) {
	if snap == nil {
		return policy.Compile(nil, policy.RoleGraph{Roles: map[policy.RoleSlug]policy.RoleNode{}}, policy.AreaTree{}, policy.ActionCatalog{})
	}

	// Build role graph from snap.Roles
	roleGraph := policy.RoleGraph{Roles: make(map[policy.RoleSlug]policy.RoleNode)}
	for _, r := range snap.GetRoles() {
		node := policy.RoleNode{Slug: policy.RoleSlug(r.GetSlug())}
		for _, inh := range r.GetInherits() {
			node.Inherits = append(node.Inherits, policy.RoleSlug(inh.GetSlug()))
		}
		roleGraph.Roles[policy.RoleSlug(r.GetSlug())] = node
	}

	// Build raw policies
	rawPolicies := make([]policy.RawPolicy, 0, len(snap.GetPolicies()))
	for _, pb := range snap.GetPolicies() {
		rp := policy.RawPolicy{Name: pb.GetName()}
		for _, subj := range pb.GetSubjects() {
			rp.Subjects = append(rp.Subjects, policy.RoleSlug(subj.GetSlug()))
		}
		for _, rule := range pb.GetAllow() {
			rp.Allow = append(rp.Allow, protoRuleToRaw(rule))
		}
		for _, rule := range pb.GetDeny() {
			rp.Deny = append(rp.Deny, protoRuleToRaw(rule))
		}
		rawPolicies = append(rawPolicies, rp)
	}

	return policy.Compile(rawPolicies, roleGraph, policy.AreaTree{}, policy.ActionCatalog{})
}

func protoRuleToRaw(r *configpb.CapabilityRule) policy.RawRule {
	verbs := make([]policy.Verb, 0, len(r.GetVerbs()))
	for _, v := range r.GetVerbs() {
		verbs = append(verbs, policy.Verb(v))
	}
	sel := policy.RawSelector{}
	if t := r.GetTargets(); t != nil {
		sel.Areas = t.GetAreas()
		sel.Classes = t.GetClasses()
		sel.EntityIDs = t.GetEntityIds()
	}
	return policy.RawRule{
		Verbs:    verbs,
		Targets:  sel,
		Services: r.GetServices(),
	}
}

// ---- pagination helpers ----

func paginateDevices(items []api.Device, page api.PageReq) ([]api.Device, api.Cursor, error) {
	start := int(page.Cursor.Position)
	if start > len(items) {
		return nil, api.Cursor{}, nil
	}
	items = items[start:]
	size := int(page.Size)
	if size <= 0 {
		size = api.DefaultPageSize
	}
	var next api.Cursor
	if len(items) > size {
		items = items[:size]
		next = api.Cursor{Position: uint64(start + size)}
	}
	return items, next, nil
}

func paginateEntities(items []api.Entity, page api.PageReq) ([]api.Entity, api.Cursor, error) {
	start := int(page.Cursor.Position)
	if start > len(items) {
		return nil, api.Cursor{}, nil
	}
	items = items[start:]
	size := int(page.Size)
	if size <= 0 {
		size = api.DefaultPageSize
	}
	var next api.Cursor
	if len(items) > size {
		items = items[:size]
		next = api.Cursor{Position: uint64(start + size)}
	}
	return items, next, nil
}

func paginateDriverInstances(items []api.DriverInstance, page api.PageReq) ([]api.DriverInstance, api.Cursor, error) {
	start := int(page.Cursor.Position)
	if start > len(items) {
		return nil, api.Cursor{}, nil
	}
	items = items[start:]
	size := int(page.Size)
	if size <= 0 {
		size = api.DefaultPageSize
	}
	var next api.Cursor
	if len(items) > size {
		items = items[:size]
		next = api.Cursor{Position: uint64(start + size)}
	}
	return items, next, nil
}

func paginateAutomations(items []api.Automation, page api.PageReq) ([]api.Automation, api.Cursor, error) {
	start := int(page.Cursor.Position)
	if start > len(items) {
		return nil, api.Cursor{}, nil
	}
	items = items[start:]
	size := int(page.Size)
	if size <= 0 {
		size = api.DefaultPageSize
	}
	var next api.Cursor
	if len(items) > size {
		items = items[:size]
		next = api.Cursor{Position: uint64(start + size)}
	}
	return items, next, nil
}

func paginateScripts(items []api.Script, page api.PageReq) ([]api.Script, api.Cursor, error) {
	start := int(page.Cursor.Position)
	if start > len(items) {
		return nil, api.Cursor{}, nil
	}
	items = items[start:]
	size := int(page.Size)
	if size <= 0 {
		size = api.DefaultPageSize
	}
	var next api.Cursor
	if len(items) > size {
		items = items[:size]
		next = api.Cursor{Position: uint64(start + size)}
	}
	return items, next, nil
}
