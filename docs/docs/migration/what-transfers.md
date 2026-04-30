# What transfers

!!! status-wip "In development"
    This feature is in active development. The `gohome import-ha` command is not yet shipped.

This page documents what the importer translates automatically, what it translates with caveats, and what it does not translate at all. Read this before running the import so you know what to expect in `IMPORT_REPORT.md`.

---

## HA construct mapping

The table below covers everything the importer reads from your Home Assistant config directory.

| HA construct | gohome target | Confidence | Notes |
|---|---|---|---|
| `configuration.yaml` | `main.pkl` + `settings.pkl` | High | Core config maps directly. Global settings (unit system, time zone, currency, home coordinates) translate to the matching gohome fields. |
| Areas (`.storage/core.area_registry`) | `areas.pkl` | High | Area names and IDs preserved. Area hierarchy detected heuristically if HA 2024+ label conventions are used; a NOTE is emitted if heuristic reasoning was applied. |
| Zones (`configuration.yaml zone:`) | `zones.pkl` | High | Lat/lon coordinates and radius preserved 1:1. |
| Device registry (`.storage/core.device_registry`) | Driver instances (diagnostic only) | Medium | gohome's device model is driver-managed; no Pkl file is written. A NOTE in `IMPORT_REPORT.md` lists detected devices so you can verify them after drivers are running. |
| Entity registry (`.storage/core.entity_registry`) | `entities/overrides.pkl` | High | Entity IDs preserved in `domain.name` format. User customisations (friendly name, area assignment, icon, hidden flag) are translated to override declarations. Actual entity existence comes from drivers. |
| Integrations (`configuration.yaml` + `.storage/core.config_entries`) | Driver instances (`drivers.pkl`) | Varies | Per-integration detail in the integration table below. |
| Automations (`automations.yaml` or `automations/*.yaml`) | `automations/<slug>.pkl` + `automations/handlers/<slug>.star` | Medium | Trigger/condition/action structure is preserved. Jinja in conditions and action templates is transpiled to Starlark. See [Jinja to Starlark](jinja-to-starlark.md) for what auto-transpiles and what emits `# FIXME`. |
| Scripts (`scripts.yaml` or `scripts/*.yaml`) | `scripts/<slug>.pkl` + `scripts/bodies/<slug>.star` | Medium | Same caveats as automations. Script parameters become typed `ScriptParam` declarations. |
| Scenes (`scenes.yaml`) | `scenes.pkl` | High | State snapshot preserved. Entity IDs and target states translated directly. Scenes contain no Jinja, so no transpilation is needed. |
| Template sensors / binary sensors (`template:` platform) | `entities/computed.pkl` (`ComputedEntity`) | Medium | Jinja value expressions are transpiled to Starlark. Expressions outside the supported Jinja subset emit `# FIXME(jinja-import)` with the original Jinja preserved. |
| Lovelace dashboards | NOT migrated | N/A | Dashboards are detected and noted in `IMPORT_REPORT.md`. Translation is a separate future milestone. Rebuild dashboards in gohome's WYSIWYG editor. |
| Users (`.storage/auth`) | `auth/users.pkl` | Medium | Display name and role (owner → admin, regular → member) translated. **Password hashes are not migrated** — re-register credentials via `gohome auth bootstrap <slug>` post-import. |
| Persons (`.storage/person`) | Merged into `auth/users.pkl` | Medium | Person display name merged into the linked user record. Device tracker associations are noted but not wired — presence is surfaced through driver entities once tracker drivers are installed. |
| Secrets (`secrets.yaml`) | `secrets.pkl` + `IMPORTED_SECRETS.env` | N/A | Secret values are never written to Pkl source. They are placed in `IMPORTED_SECRETS.env` (gitignored). `secrets.pkl` emits `read("env:UPPER_SNAKE_CASE")` references. Delete `IMPORTED_SECRETS.env` after sourcing it. |

### Confidence levels

- **High** — the construct translates directly; expect no manual intervention under normal conditions.
- **Medium** — the translation is structurally complete but may require follow-up (re-entering credentials, reviewing `# FIXME` markers, or re-commissioning hardware).
- **Low** — approximate translation; significant manual review required.
- **N/A** — not translated; handled via notes or a separate workflow.

---

## Integration coverage

The importer ships focused, field-by-field mappers for the integrations in gohome's v1.0 driver set. For everything else it emits a labelled placeholder that preserves the original HA YAML so you can apply it manually once a driver is available.

| HA integration | gohome driver | Confidence | Notes |
|---|---|---|---|
| MQTT (`mqtt`) | `driver.mqtt` | High | 1:1 field mapping. Broker host, port, username, TLS settings all preserved. Password translated to a secret reference. |
| Zigbee2MQTT (`zigbee2mqtt`) | `driver.zigbee2mqtt` | High | Bridge address, topic prefix, and device list preserved. The bridge itself (MQTT broker) must already be running. |
| ESPHome (`esphome`) | `driver.esphome` | High | Device addresses and API encryption keys preserved. Encryption keys become secret references in `secrets.pkl`. |
| HomeKit Controller (`homekit`) | `driver.homekit` | Medium | Pairing codes cannot be migrated. The driver instance is created with the bridge address, but re-pairing is required from the gohome side. |
| Matter (`matter`) | `driver.matter` | Medium | Commission codes cannot be migrated. The driver instance is created, but re-commissioning is required for each Matter device. |
| Hue (`hue`) | `driver.hue` | High | Bridge IP and API key preserved. The API key becomes a secret reference. A NOTE reminds you to verify the bridge address has not changed. |
| Nest (`nest`) | `driver.nest` | Medium | OAuth tokens cannot be migrated. The driver instance is created with your project ID and device access credentials, but you must re-authorise via OAuth after the driver starts. |
| Z-Wave JS (`zwave_js`) | `driver.zwave` | High | USB device path preserved. Network key (S0/S2) becomes a secret reference. Z-Wave JS must be running independently; gohome connects to its WebSocket interface. |
| Generic REST (`rest`) | `driver.rest` | Medium | Resource URLs, scan intervals, and value templates preserved. Authentication headers are translated to secret references. Complex value templates may emit `# FIXME(jinja-import)`. |
| Generic webhook (`webhook`) | `driver.webhook` | Medium | Webhook IDs are preserved where possible, but the webhook base URL changes to your gohome instance. Update any external services that send to the old HA webhook URL. |
| Template platform (`template`) | `ComputedEntity` | Medium | See the `entities/computed.pkl` row in the construct table above. Handled by the template mapper, not a driver instance. |
| Custom integrations (HACS) | NOT supported | N/A | No mapper exists. The importer emits a `# FIXME(unmapped-integration)` placeholder with the original YAML preserved. |
| Other integrations | NOT supported | N/A | Integrations outside gohome's v1.0 driver set produce `# FIXME(unmapped-integration)` placeholders. Items remain as skeleton config entries for manual wiring once a driver exists. |

### How placeholders work

When the importer encounters an integration it cannot map, it writes a placeholder into `drivers.pkl`:

```pkl
// FIXME(unmapped-integration): integration 'sonoff_lan' is outside gohome's v1.0
// driver set. No mapper exists. Original HA configuration preserved below:
//
//   platform: sonoff_lan
//   username: myuser
//   password: !secret sonoff_password
new gohome.imported.UnmappedIntegration {
  sourceName = "sonoff_lan"
  sourceConfigYaml = #"""
  platform: sonoff_lan
  username: myuser
  password: !secret sonoff_password
  """#
}
```

Search for `FIXME(` in the output directory to find all items that need attention. The `IMPORT_REPORT.md` file lists every FIXME with its file location.

---

## Automation trigger mapping

| HA trigger platform | gohome trigger | Notes |
|---|---|---|
| `state` | `StateChangedTrigger` | Direct mapping. `to` / `from` / `for` fields preserved. |
| `numeric_state` | `StateChangedTrigger` + condition | Threshold wrapped as a Starlark condition on the trigger. |
| `time` | `TimeTrigger` | Direct mapping. |
| `time_pattern` | `TimeTrigger` (cron form) | HA time patterns are cron-equivalent; mapped directly. |
| `template` | `StateChangedTrigger` + Starlark condition | Jinja value expression transpiled to Starlark. |
| `event` | `EventTrigger` | Direct mapping by event kind. |
| `homeassistant` (start/stop) | `EventTrigger` (`system_started` / `system_stopping`) | Direct mapping to gohome system events. |
| `mqtt` | `EventTrigger` (`driver:mqtt`) | NOTE: requires the MQTT driver to be configured. |
| `webhook` | `EventTrigger` (`driver:webhook`) | NOTE: webhook URL changes to your gohome instance. |
| `sun` | `EventTrigger` (`sun:rise` / `sun:set`) | Requires the sun driver. |
| `zone` / `geo_location` | `EventTrigger` | Requires a presence driver to emit zone events. |
| Others | `# FIXME(unmapped-trigger)` | Original YAML preserved in the comment. |

## Automation action mapping

| HA action | gohome action | Notes |
|---|---|---|
| `service: light.turn_on` | `CallServiceAction` → `entity.<id>.turn_on(...)` | Translated to a typed capability call. |
| `service: <domain>.<svc>` (no entity selector) | `# FIXME(unmapped-action)` | No entity target to resolve the capability against. |
| `delay: ...` | `SleepAction` | Direct mapping. |
| `wait_template: ...` | `WaitUntilAction` with Starlark | Jinja condition transpiled to Starlark. |
| `condition: ...` (mid-action) | Inline `if not (...): return` | Direct mapping. |
| `repeat: ...` | `for` / `while` in Starlark | Depends on repeat shape; fixed-count → `range()`, until → `while`. |
| `choose: ...` | `if/elif/else` chain | Direct mapping. |
| `parallel: ...` | `# FIXME(unmapped-action)` | No parallel action in gohome v1.0. |
| `event: ...` | `FireEventAction` | Direct mapping. |
| `scene: ...` | `ApplySceneAction` | Direct mapping by scene slug. |
| `script: ...` | `CallScriptAction` | Direct mapping by script slug. |
