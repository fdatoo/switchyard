package observability

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics groups every Prometheus collector registered by the daemon.
type Metrics struct {
	Registry *prometheus.Registry

	// Append path
	EventsAppended *prometheus.CounterVec
	AppendDuration prometheus.Histogram
	AppendRetries  prometheus.Counter
	AppendFailures *prometheus.CounterVec

	// Projectors
	ProjectorApplyDuration *prometheus.HistogramVec
	ProjectorFailures      *prometheus.CounterVec
	ProjectorLag           *prometheus.GaugeVec
	ProjectorCatchup       *prometheus.GaugeVec

	// Tailer
	TailerLag       prometheus.Gauge
	TailerBatchSize prometheus.Histogram

	// Subscriptions
	SubscriptionActive    *prometheus.GaugeVec
	SubscriptionDelivered *prometheus.CounterVec
	SubscriptionDropped   *prometheus.CounterVec
	SubscriptionBuffered  *prometheus.GaugeVec
	SubscriptionCatchup   *prometheus.HistogramVec

	// Snapshots
	SnapshotDuration   *prometheus.HistogramVec
	SnapshotSize       *prometheus.GaugeVec
	SnapshotLastPos    *prometheus.GaugeVec
	SnapshotCorruption *prometheus.CounterVec

	// Storage
	SQLiteWALBytes    prometheus.Gauge
	SQLiteEventsTotal prometheus.Gauge
	SQLiteBusyRetries prometheus.Counter

	// Startup
	StartupPhase          prometheus.Gauge
	StartupDuration       prometheus.Histogram
	ReplayEventsProcessed prometheus.Counter
	RecoveryModeEntered   prometheus.Counter

	// Health
	BuildInfo *prometheus.GaugeVec
	Uptime    prometheus.GaugeFunc

	// Carport (driver subsystem)
	CarportDriverInstances        *prometheus.GaugeVec
	CarportHandshakesTotal        *prometheus.CounterVec
	CarportCommandDispatchTotal   *prometheus.CounterVec
	CarportCommandDispatchSeconds *prometheus.HistogramVec
	CarportEventsIngestedTotal    *prometheus.CounterVec
	CarportDriverRestartsTotal    *prometheus.CounterVec
	CarportHealthProbeSeconds     *prometheus.HistogramVec
	CarportStreamMessagesTotal    *prometheus.CounterVec
	CarportPendingCommands        *prometheus.GaugeVec

	// Automation + script (C6)
	AutomationTriggersTotal       *prometheus.CounterVec
	AutomationRunsTotal           *prometheus.CounterVec
	AutomationConditionsTotal     *prometheus.CounterVec
	AutomationActionsTotal        *prometheus.CounterVec
	AutomationReloadFailuresTotal prometheus.Counter
	ScriptInvocationsTotal        *prometheus.CounterVec

	AutomationRunDurationSeconds *prometheus.HistogramVec
	AutomationStarlarkSteps      *prometheus.HistogramVec
	ScriptDurationSeconds        *prometheus.HistogramVec

	AutomationInflight   *prometheus.GaugeVec
	AutomationRegistered prometheus.Gauge
	ScriptRegistered     prometheus.Gauge

	// API (C7)
	APIRequestsTotal                 *prometheus.CounterVec
	APIRequestDurationSeconds        *prometheus.HistogramVec
	APIStreamEventsSentTotal         *prometheus.CounterVec
	APIStreamHeartbeatsSentTotal     *prometheus.CounterVec
	APIStreamBackpressureClosesTotal *prometheus.CounterVec
	APIWebhookReceivedTotal          *prometheus.CounterVec
	APIActiveStreams                 *prometheus.GaugeVec

	// MCP (C8)
	MCPToolCallsTotal              *prometheus.CounterVec
	MCPToolCallDuration            *prometheus.HistogramVec
	MCPResourceSubscriptionsActive *prometheus.GaugeVec
	MCPResourceUpdatesSent         *prometheus.CounterVec
	MCPResourceOverflowCloses      *prometheus.CounterVec
	MCPEvalStarlarkTruncated       prometheus.Counter
	MCPConfigFileWrites            *prometheus.CounterVec

	// Auth + Policy (C9)
	AuthLoginAttemptsTotal   *prometheus.CounterVec
	AuthLoginDurationSeconds *prometheus.HistogramVec
	// AuthActiveSessions counts active cookie sessions; wired by sessions.Store on issue/expire.
	AuthActiveSessions prometheus.Gauge
	// AuthActiveTokens counts non-revoked, non-expired tokens; wired by credentials.Tokens on issue/revoke.
	AuthActiveTokens               prometheus.Gauge
	AuthThrottleBlocksTotal        *prometheus.CounterVec
	PolicyCompileDurationSeconds   prometheus.Histogram
	PolicyCompileGeneration        prometheus.Gauge
	PolicyAuthorizeTotal           *prometheus.CounterVec
	PolicyAuthorizeDurationSeconds prometheus.Histogram
}

// NewMetrics creates an isolated registry and registers all daemon collectors.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{Registry: reg}

	m.EventsAppended = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "switchyard_events_appended_total", Help: "Events appended by kind"},
		[]string{"kind"},
	)
	m.AppendDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "switchyard_events_append_duration_seconds",
		Help:    "End-to-end Append duration",
		Buckets: prometheus.DefBuckets,
	})
	m.AppendRetries = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "switchyard_events_append_retries_total", Help: "SQLite BUSY retries on Append",
	})
	m.AppendFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "switchyard_events_append_failures_total", Help: "Append failures by stage"},
		[]string{"stage"},
	)

	m.ProjectorApplyDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "switchyard_projector_apply_duration_seconds", Help: "Projector.Apply duration"},
		[]string{"projector", "mode"},
	)
	m.ProjectorFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "switchyard_projector_failures_total", Help: "Projector.Apply failures"},
		[]string{"projector", "mode"},
	)
	m.ProjectorLag = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "switchyard_projector_lag_events", Help: "Events behind head per projector"},
		[]string{"projector"},
	)
	m.ProjectorCatchup = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "switchyard_projector_catchup_mode", Help: "1 if async projector is in SQL catchup"},
		[]string{"projector"},
	)

	m.TailerLag = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "switchyard_tailer_lag_events", Help: "Events tailer is behind head",
	})
	m.TailerBatchSize = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "switchyard_tailer_batch_size", Help: "Events per tailer dispatch batch",
		Buckets: []float64{1, 5, 10, 50, 100, 500, 1000},
	})

	m.SubscriptionActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "switchyard_subscription_active", Help: "1 if subscription is active"},
		[]string{"name"},
	)
	m.SubscriptionDelivered = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "switchyard_subscription_delivered_total", Help: "Events delivered to subscriber"},
		[]string{"name"},
	)
	m.SubscriptionDropped = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "switchyard_subscription_dropped_total", Help: "Events dropped for slow subscribers"},
		[]string{"name"},
	)
	m.SubscriptionBuffered = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "switchyard_subscription_buffered", Help: "Events buffered per subscriber"},
		[]string{"name"},
	)
	m.SubscriptionCatchup = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "switchyard_subscription_catchup_duration_seconds", Help: "Catchup phase duration"},
		[]string{"name"},
	)

	m.SnapshotDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "switchyard_snapshot_duration_seconds", Help: "Snapshot write duration"},
		[]string{"owner"},
	)
	m.SnapshotSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "switchyard_snapshot_size_bytes", Help: "Latest snapshot size"},
		[]string{"owner"},
	)
	m.SnapshotLastPos = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "switchyard_snapshot_last_position", Help: "Latest snapshot position"},
		[]string{"owner"},
	)
	m.SnapshotCorruption = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "switchyard_snapshot_corruption_total", Help: "Corrupt snapshots encountered"},
		[]string{"owner"},
	)

	m.SQLiteWALBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "switchyard_sqlite_wal_bytes", Help: "Current WAL size",
	})
	m.SQLiteEventsTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "switchyard_sqlite_events_total", Help: "Rows in events table",
	})
	m.SQLiteBusyRetries = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "switchyard_sqlite_busy_retries_total", Help: "SQLITE_BUSY retries across all callers",
	})

	m.StartupPhase = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "switchyard_startup_phase", Help: "Current startup phase 1-5; 0 = not started",
	})
	m.StartupDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "switchyard_startup_duration_seconds", Help: "Time to reach phase 5",
	})
	m.ReplayEventsProcessed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "switchyard_replay_events_processed_total", Help: "Events replayed at startup",
	})
	m.RecoveryModeEntered = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "switchyard_recovery_mode_entered_total", Help: "Times recovery mode was entered",
	})

	m.BuildInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "switchyard_build_info", Help: "Build metadata"},
		[]string{"version", "commit", "goversion"},
	)

	m.CarportDriverInstances = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "carport_driver_instances", Help: "Driver instances by FSM state"},
		[]string{"state"},
	)
	m.CarportHandshakesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "carport_handshakes_total", Help: "Carport Handshake outcomes"},
		[]string{"instance_id", "result"},
	)
	m.CarportCommandDispatchTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "carport_command_dispatch_total", Help: "Dispatch outcomes"},
		[]string{"instance_id", "result"},
	)
	m.CarportCommandDispatchSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "carport_command_dispatch_duration_seconds", Help: "Dispatch latency"},
		[]string{"instance_id", "capability"},
	)
	m.CarportEventsIngestedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "carport_events_ingested_total", Help: "Events ingested from drivers"},
		[]string{"instance_id", "kind"},
	)
	m.CarportDriverRestartsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "carport_driver_restarts_total", Help: "Driver restarts by cause"},
		[]string{"instance_id", "reason"},
	)
	m.CarportHealthProbeSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "carport_health_probe_duration_seconds", Help: "Health probe latency"},
		[]string{"instance_id"},
	)
	m.CarportStreamMessagesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "carport_stream_messages_received_total", Help: "DriverToHost messages by kind"},
		[]string{"instance_id", "kind"},
	)
	m.CarportPendingCommands = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "carport_pending_commands", Help: "In-flight Dispatch calls by instance"},
		[]string{"instance_id"},
	)

	m.AutomationTriggersTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "switchyard_automation_triggers_total", Help: "Automation trigger fires admitted to execution"},
		[]string{"automation_id", "trigger_kind"},
	)
	m.AutomationRunsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "switchyard_automation_runs_total", Help: "Automation run completions by outcome"},
		[]string{"automation_id", "outcome"},
	)
	m.AutomationConditionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "switchyard_automation_conditions_total", Help: "Automation condition evaluations by result"},
		[]string{"automation_id", "result"},
	)
	m.AutomationActionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "switchyard_automation_actions_total", Help: "Automation action executions by kind and result"},
		[]string{"automation_id", "action_kind", "result"},
	)
	m.AutomationReloadFailuresTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "switchyard_automation_reload_failures_total", Help: "Automation config reload compile failures",
	})
	m.ScriptInvocationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "switchyard_script_invocations_total", Help: "Script invocations by outcome and invoker kind"},
		[]string{"script_name", "outcome", "invoked_by_kind"},
	)

	m.AutomationRunDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "switchyard_automation_run_duration_seconds", Help: "Automation run wall-clock duration", Buckets: prometheus.DefBuckets},
		[]string{"automation_id"},
	)
	m.AutomationStarlarkSteps = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "switchyard_automation_starlark_steps",
			Help:    "Starlark steps executed per automation run",
			Buckets: []float64{100, 1000, 10_000, 100_000, 1_000_000, 10_000_000},
		},
		[]string{"automation_id"},
	)
	m.ScriptDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "switchyard_script_duration_seconds", Help: "Script execution wall-clock duration", Buckets: prometheus.DefBuckets},
		[]string{"script_name"},
	)

	m.AutomationInflight = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "switchyard_automation_inflight", Help: "Automation runs currently in flight"},
		[]string{"automation_id"},
	)
	m.AutomationRegistered = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "switchyard_automation_registered", Help: "Number of registered automations",
	})
	m.ScriptRegistered = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "switchyard_script_registered", Help: "Number of registered scripts",
	})

	m.APIRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "switchyard_api_requests_total", Help: "Completed API RPCs by procedure and code."},
		[]string{"procedure", "code"},
	)
	m.APIRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "switchyard_api_request_duration_seconds",
			Help:    "Latency of completed API RPCs.",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 14),
		},
		[]string{"procedure", "code"},
	)
	m.APIStreamEventsSentTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "switchyard_api_stream_events_sent_total", Help: "Streamed payload events sent."},
		[]string{"procedure"},
	)
	m.APIStreamHeartbeatsSentTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "switchyard_api_stream_heartbeats_sent_total", Help: "Heartbeats sent on streaming RPCs."},
		[]string{"procedure"},
	)
	m.APIStreamBackpressureClosesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "switchyard_api_stream_backpressure_closes_total", Help: "Streaming RPCs closed with RESOURCE_EXHAUSTED."},
		[]string{"procedure"},
	)
	m.APIWebhookReceivedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "switchyard_api_webhook_received_total", Help: "Webhook outcomes by slug and result."},
		[]string{"slug", "result"},
	)
	m.APIActiveStreams = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "switchyard_api_active_streams", Help: "Currently-open server streams."},
		[]string{"procedure"},
	)

	m.MCPToolCallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "switchyard_mcp_tool_calls_total", Help: "Total MCP tool dispatches by tool and outcome."},
		[]string{"tool", "result"},
	)
	m.MCPToolCallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "switchyard_mcp_tool_call_duration_seconds", Help: "Latency of MCP tool dispatches.", Buckets: prometheus.DefBuckets},
		[]string{"tool", "result"},
	)
	m.MCPResourceSubscriptionsActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "switchyard_mcp_resource_subscriptions_active", Help: "Currently-open MCP resource subscriptions."},
		[]string{"kind"},
	)
	m.MCPResourceUpdatesSent = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "switchyard_mcp_resource_updates_sent_total", Help: "MCP notifications/resources/updated fired."},
		[]string{"kind"},
	)
	m.MCPResourceOverflowCloses = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "switchyard_mcp_resource_overflow_closes_total", Help: "MCP subscriptions affected by buffer overflow."},
		[]string{"kind", "reason"},
	)
	m.MCPEvalStarlarkTruncated = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "switchyard_mcp_eval_starlark_truncated_total", Help: "eval_starlark calls whose output exceeded the cap.",
	})
	m.MCPConfigFileWrites = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "switchyard_mcp_config_file_writes_total", Help: "Filesystem-tool writes by extension and outcome."},
		[]string{"extension", "result"},
	)

	m.AuthLoginAttemptsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "switchyard_auth_login_attempts_total",
		Help: "Login attempts by method and result.",
	}, []string{"method", "result"})
	m.AuthLoginDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "switchyard_auth_login_duration_seconds",
		Help:    "Login latency by method.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method"})
	m.AuthActiveSessions = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "switchyard_auth_active_sessions",
		Help: "Active cookie sessions.",
	})
	m.AuthActiveTokens = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "switchyard_auth_active_tokens",
		Help: "Non-revoked, non-expired tokens.",
	})
	m.AuthThrottleBlocksTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "switchyard_auth_throttle_blocks_total",
		Help: "Login attempts blocked by throttle.",
	}, []string{"method"})
	m.PolicyCompileDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "switchyard_policy_compile_duration_seconds",
		Help:    "Policy compile latency.",
		Buckets: prometheus.DefBuckets,
	})
	m.PolicyCompileGeneration = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "switchyard_policy_compile_generation",
		Help: "Current compiled policy generation.",
	})
	m.PolicyAuthorizeTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "switchyard_policy_authorize_total",
		Help: "Authorize decisions.",
	}, []string{"result", "sub_reason"})
	m.PolicyAuthorizeDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "switchyard_policy_authorize_duration_seconds",
		Help:    "Authorize latency.",
		Buckets: prometheus.DefBuckets,
	})

	reg.MustRegister(
		m.EventsAppended, m.AppendDuration, m.AppendRetries, m.AppendFailures,
		m.ProjectorApplyDuration, m.ProjectorFailures, m.ProjectorLag, m.ProjectorCatchup,
		m.TailerLag, m.TailerBatchSize,
		m.SubscriptionActive, m.SubscriptionDelivered, m.SubscriptionDropped, m.SubscriptionBuffered, m.SubscriptionCatchup,
		m.SnapshotDuration, m.SnapshotSize, m.SnapshotLastPos, m.SnapshotCorruption,
		m.SQLiteWALBytes, m.SQLiteEventsTotal, m.SQLiteBusyRetries,
		m.StartupPhase, m.StartupDuration, m.ReplayEventsProcessed, m.RecoveryModeEntered,
		m.BuildInfo,
		m.CarportDriverInstances, m.CarportHandshakesTotal, m.CarportCommandDispatchTotal,
		m.CarportCommandDispatchSeconds, m.CarportEventsIngestedTotal, m.CarportDriverRestartsTotal,
		m.CarportHealthProbeSeconds, m.CarportStreamMessagesTotal, m.CarportPendingCommands,
		m.AutomationTriggersTotal, m.AutomationRunsTotal, m.AutomationConditionsTotal,
		m.AutomationActionsTotal, m.AutomationReloadFailuresTotal, m.ScriptInvocationsTotal,
		m.AutomationRunDurationSeconds, m.AutomationStarlarkSteps, m.ScriptDurationSeconds,
		m.AutomationInflight, m.AutomationRegistered, m.ScriptRegistered,
		m.APIRequestsTotal, m.APIRequestDurationSeconds,
		m.APIStreamEventsSentTotal, m.APIStreamHeartbeatsSentTotal,
		m.APIStreamBackpressureClosesTotal, m.APIWebhookReceivedTotal,
		m.APIActiveStreams,
		m.MCPToolCallsTotal, m.MCPToolCallDuration,
		m.MCPResourceSubscriptionsActive, m.MCPResourceUpdatesSent,
		m.MCPResourceOverflowCloses, m.MCPEvalStarlarkTruncated,
		m.MCPConfigFileWrites,
		m.AuthLoginAttemptsTotal, m.AuthLoginDurationSeconds,
		m.AuthActiveSessions, m.AuthActiveTokens,
		m.AuthThrottleBlocksTotal,
		m.PolicyCompileDurationSeconds, m.PolicyCompileGeneration,
		m.PolicyAuthorizeTotal, m.PolicyAuthorizeDurationSeconds,
	)
	return m
}

// SetBuildInfo publishes immutable build metadata as a Prometheus gauge label set.
func (m *Metrics) SetBuildInfo(version, commit, goVersion string) {
	m.BuildInfo.WithLabelValues(version, commit, goVersion).Set(1)
}
