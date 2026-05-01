# First-Party Driver Catalog

This page documents the first-party drivers shipped with gohome v1.0. All drivers are open-source and hosted under the `fynn-labs` GitHub organisation.

For driver installation instructions see [Using drivers](index.md).

---

### MQTT (`driver.mqtt`)

!!! status-alpha "Alpha — shipped, interface evolving"

A general-purpose MQTT publish/subscribe driver. Register any topic as an entity; the driver maps incoming payloads to state changes and outgoing commands to MQTT publishes. Useful for devices that speak MQTT natively but don't have a dedicated gohome driver.

**Config fields**

| Field | Type | Required | Description |
|---|---|---|---|
| `broker_url` | `string` | yes | MQTT broker URL, e.g. `mqtt://192.168.1.10:1883` |
| `client_id` | `string` | no | Client ID sent to broker. Defaults to `gohome-<instance>` |
| `username` | `string` | no | MQTT username |
| `password_env` | `string` | no | Name of an env var containing the MQTT password |
| `tls_ca_cert` | `string` | no | Path to CA certificate for TLS brokers |
| `qos` | `int` | no | Default QoS level (0, 1, or 2). Defaults to `1` |
| `entities` | `list` | yes | List of entity definitions (topic, payload template, capabilities) |

**Known caveats**

- Retained messages are replayed on connect; entities show their last-known state immediately on driver startup.
- QoS 2 is supported but adds latency on high-traffic topics; prefer QoS 1 for most home automation use cases.
- No built-in payload schema validation; malformed payloads are logged and dropped.

[Source repo](https://github.com/fynn-labs/driver-mqtt)

---

### Zigbee2MQTT (`driver.z2m`)

!!! status-alpha "Alpha — shipped, interface evolving"

Mirrors a [Zigbee2MQTT](https://www.zigbee2mqtt.io/) deployment into gohome over the MQTT broker that Z2M publishes to. Discovers all paired devices on startup, then reconciles live via the retained `bridge/devices` topic. v0.1 surfaces three device classes: lights (`light.*`), numeric sensors (`numeric_sensor.*`), and binary sensors (`binary_sensor.*`).

**Config fields**

| Field | Type | Required | Description |
|---|---|---|---|
| `broker_url` | `string` | yes | `tcp://host:1883` or `ssl://host:8883` |
| `username` | `string` | no | MQTT broker username |
| `password_env` | `string` | no | Env var containing the MQTT password |
| `base_topic` | `string` | no | Z2M's `mqtt.base_topic` setting (default `zigbee2mqtt`) |
| `client_id` | `string` | no | MQTT client identifier (default `gohome-z2m-<random8>`) |
| `tls_skip_verify` | `bool` | no | Skip TLS verification (default `false`) |

**Known caveats**

- New devices paired in Z2M are picked up automatically; no driver restart needed.
- Smart-plug actuators (writable `state`) are out of scope in v0.1 — read-only sub-properties (`power`, `energy`) still surface.
- Per-device availability depends on Z2M's availability feature being enabled server-side; otherwise entities default to `Available=true`.
- `/set` publishes are best-effort: a successful publish is reported as `ok=true` even if Z2M silently ignores the command (no MQTT 5 request/response in v0.1).

[Source repo](https://github.com/fdatoo/gohome/tree/main/drivers/z2m)

---

### Matter (`driver.matter`)

!!! status-wip "In development"
    This driver is in active development and not yet available.

Native Matter/Thread protocol integration. Commissions Matter devices directly without a separate bridge. Supports lights, switches, sensors, and thermostats in the Matter 1.x device type library.

**Config fields**

| Field | Type | Required | Description |
|---|---|---|---|
| `fabric_id` | `string` | yes | Matter fabric identifier for this gohome instance |
| `storage_path` | `string` | yes | Path to Matter credential/fabric storage directory |
| `interface` | `string` | no | Network interface to use for mDNS discovery. Defaults to auto-detect |
| `thread_dataset` | `string` | no | Thread active operational dataset (hex-encoded) for Thread-over-Thread devices |

**Known caveats**

- Thread border router must be running separately (e.g. Apple HomePod, Google Home Hub, or an open-source BR).
- Matter commissioning is initiated via `gohome command send` — a dedicated commissioning UI is planned for v1.1.

[Source repo](https://github.com/fynn-labs/driver-matter)

---

### HomeKit Bridge (`driver.homekit`)

!!! status-wip "In development"
    This driver is in active development and not yet available.

Exposes gohome entities to Apple HomeKit. Appears as an accessory bridge in the Home app; individual gohome entities are mapped to HomeKit accessory types. Supports lights, switches, sensors, thermostats, locks, and covers.

**Config fields**

| Field | Type | Required | Description |
|---|---|---|---|
| `pin` | `string` | yes | HomeKit pairing PIN (format: `XXX-XX-XXX`) |
| `port` | `int` | no | HAP server port. Defaults to `51826` |
| `storage_path` | `string` | yes | Path to HomeKit pairing data storage directory |
| `entities` | `list` | no | Explicit list of entity IDs to expose. Defaults to all compatible entities |

**Known caveats**

- HomeKit requires mDNS on the local network; does not work across VLANs without mDNS reflection.
- Entity updates from HomeKit are authoritative — commands from Apple Home will override gohome state.

[Source repo](https://github.com/fynn-labs/driver-homekit)

---

### ESPHome Native (`driver.esphome`)

!!! status-alpha "Alpha — shipped, interface evolving"

Connects to [ESPHome](https://esphome.io/) devices using the ESPHome native API. No MQTT broker required — the driver communicates directly with device firmware over TCP. Auto-discovers entities from the device's API and maps them to gohome entity types.

**Config fields**

| Field | Type | Required | Description |
|---|---|---|---|
| `host` | `string` | yes | Hostname or IP address of the ESPHome device |
| `port` | `int` | no | Native API port. Defaults to `6053` |
| `password_env` | `string` | no | Env var containing the API password (if set in ESPHome config) |
| `encryption_key_env` | `string` | no | Env var containing the API encryption key (ESPHome `api.encryption.key`) |
| `reconnect_interval_s` | `int` | no | Seconds between reconnect attempts. Defaults to `5` |

**Known caveats**

- One driver instance per ESPHome device; configure multiple instances in `drivers.toml` for multiple devices.
- The native API is not yet fully stable in ESPHome for all entity types; sensor-only entities are most reliable.
- Encryption requires ESPHome firmware 2022.6 or later.

[Source repo](https://github.com/fynn-labs/driver-esphome)

---

### Z-Wave JS (`driver.zwave`)

!!! status-wip "In development"
    This driver is in active development and not yet available.

Integrates with a [Z-Wave JS](https://zwave-js.github.io/node-zwave-js/) server. The driver connects to the Z-Wave JS WebSocket API to discover and control Z-Wave devices. Requires a Z-Wave JS server running separately (e.g. via Docker).

**Config fields**

| Field | Type | Required | Description |
|---|---|---|---|
| `server_url` | `string` | yes | Z-Wave JS server WebSocket URL, e.g. `ws://localhost:3000` |
| `schema_version` | `int` | no | Z-Wave JS schema version. Defaults to current supported version |
| `network_key_env` | `string` | no | Env var containing the Z-Wave network security key |

**Known caveats**

- Requires a dedicated Z-Wave USB stick and a running Z-Wave JS server.
- Node inclusion/exclusion must be done via the Z-Wave JS UI or API; the driver does not expose inclusion mode as a capability.

[Source repo](https://github.com/fynn-labs/driver-zwave)

---

### Generic REST (`driver.rest`)

!!! status-alpha "Alpha — shipped, interface evolving"

Polls HTTP endpoints on a schedule or accepts webhook triggers, and maps responses to entity state changes. Useful for cloud APIs, custom HTTP devices, or services without a dedicated driver.

**Config fields**

| Field | Type | Required | Description |
|---|---|---|---|
| `entities` | `list` | yes | Entity definitions including poll URL, interval, and response mapping |
| `default_headers` | `map` | no | HTTP headers sent with every request (e.g. `Authorization`) |
| `timeout_s` | `int` | no | HTTP request timeout in seconds. Defaults to `10` |
| `tls_skip_verify` | `bool` | no | Skip TLS certificate verification. Not recommended for production |

Each entity in `entities` supports:

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | `string` | yes | Entity ID in gohome (e.g. `sensor.weather_temp`) |
| `poll_url` | `string` | no | URL to poll. Omit if using webhook mode only |
| `poll_interval_s` | `int` | no | Poll interval in seconds. Required if `poll_url` is set |
| `method` | `string` | no | HTTP method for poll requests. Defaults to `GET` |
| `state_jq` | `string` | no | jq expression to extract the state value from the response body |

**Known caveats**

- jq is evaluated server-side; complex expressions may have performance implications at short poll intervals.
- No built-in authentication flows (OAuth etc.) — use `default_headers` with a pre-obtained token.

[Source repo](https://github.com/fynn-labs/driver-rest)

---

### Generic Webhook (`driver.webhook`)

!!! status-alpha "Alpha — shipped, interface evolving"

Receives HTTP webhooks from external services and maps incoming payloads to gohome state changes or events. Acts as a passive inbound endpoint — no polling. Useful for services that push events (cloud APIs, IFTTT, custom hardware).

**Config fields**

| Field | Type | Required | Description |
|---|---|---|---|
| `listen_port` | `int` | yes | Port to listen on for incoming webhooks |
| `path_prefix` | `string` | no | URL path prefix for all webhook endpoints. Defaults to `/webhook` |
| `secret_env` | `string` | no | Env var containing a shared HMAC secret for request verification |
| `endpoints` | `list` | yes | List of endpoint definitions mapping paths to entity state updates |

**Known caveats**

- The webhook server listens on the host network; ensure the port is reachable from external services.
- HMAC verification is optional but strongly recommended for internet-facing webhooks.
- No built-in TLS — place behind a reverse proxy (nginx, Caddy) for HTTPS.

[Source repo](https://github.com/fynn-labs/driver-webhook)

---

### Nest (`driver.nest`)

!!! status-wip "In development"
    This driver is in active development and not yet available.

Integrates Google Nest thermostats and cameras via the Google Nest Device Access API. Exposes thermostats as climate entities and Nest cameras as binary-sensor/camera entities.

**Config fields**

| Field | Type | Required | Description |
|---|---|---|---|
| `project_id` | `string` | yes | Google Device Access project ID |
| `client_id` | `string` | yes | OAuth 2.0 client ID from Google Cloud Console |
| `client_secret_env` | `string` | yes | Env var containing the OAuth 2.0 client secret |
| `refresh_token_env` | `string` | yes | Env var containing the OAuth 2.0 refresh token |
| `poll_interval_s` | `int` | no | Polling interval for thermostat state. Defaults to `60` |

**Known caveats**

- Requires a Google Device Access account (one-time $5 registration fee as of 2026).
- Camera streaming is read-only; no two-way audio support in v1.0.
- Google's API rate limits apply; very frequent polling will result in 429 errors.

[Source repo](https://github.com/fynn-labs/driver-nest)

---

### Hue (`driver.hue`)

!!! status-alpha "Alpha — shipped, interface evolving"

Mirrors all lights on a Philips Hue bridge into gohome as `light.*` entities using the local CLIP v2 API — no cloud, no Philips account required. Supports `turn_on`, `turn_off`, `set_brightness`, and `set_color_temp` for white and tunable-white control. Color (RGB/XY), groups, scenes, and sensors are out of scope for v0.1.

**Config fields**

| Field | Type | Required | Description |
|---|---|---|---|
| `bridge_address` | `string` | yes | IP address or hostname of the Hue bridge |
| `api_key_env` | `string` | yes | Env var containing the Hue API key (obtained via the curl recipe in the driver README) |
| `tls_skip_verify` | `bool` | no | Skip TLS verification for the CLIP v2 HTTPS endpoint. Defaults to `true` (bridge uses self-signed cert) |

**Known caveats**

- CLIP v2 only — bridges on firmware older than 1.48 (pre-2021) are not supported.
- Server-sent events deliver state changes from wall switches and the Hue app to gohome with sub-second latency.
- The Hue bridge API key is per-application; store it in a secret and reference via `api_key_env`.

[Source repo](https://github.com/fynn-labs/driver-hue)
