# driver-z2m

GoHome Carport driver for [Zigbee2MQTT](https://www.zigbee2mqtt.io/).

- One driver instance = one Z2M deployment (= one MQTT broker hosting it).
- Discovers all paired devices automatically; hot add/remove is live.
- Surfaces three device classes in v0.1: lights, numeric sensors, binary sensors.

## Quick start

### 1. Reach your Z2M's MQTT broker

The driver does not talk to Z2M directly — it talks to whatever MQTT broker Z2M publishes to (Mosquitto, EMQX, NanoMQ, etc.). You need:

- Broker URL (`tcp://host:1883` or `ssl://host:8883`).
- Optional username/password.
- The base topic Z2M is configured with — almost always `zigbee2mqtt`, configurable in Z2M's `configuration.yaml` under `mqtt.base_topic`.

### 2. Configure the driver

The driver receives its instance config as a JSON blob in the `GOHOME_CARPORT_INSTANCE_CONFIG` environment variable, which `gohomed` populates from the carport instance's `config_json` TOML field.

| Field | Type | Required | Default | Purpose |
|---|---|---|---|---|
| `broker_url` | string | yes | — | `tcp://host:1883` or `ssl://host:8883` |
| `username` | string | no | — | MQTT broker username |
| `password_env` | string | no | — | Name of an env var holding the MQTT password (referenced indirectly so secrets stay out of config files) |
| `base_topic` | string | no | `zigbee2mqtt` | Z2M's `mqtt.base_topic` setting |
| `client_id` | string | no | `gohome-z2m-<random8>` | MQTT client identifier |
| `tls_skip_verify` | bool | no | `false` | Skip TLS verification (self-signed brokers) |

Example carport instance TOML:

```toml
[[carport.instances]]
id = "z2m-home"
binary = "/usr/local/bin/z2m-driver"
config_json = '''
{
  "broker_url": "tcp://10.0.0.5:1883",
  "username": "gohome",
  "password_env": "Z2M_PASSWORD"
}
'''
```

Operational env vars (independent of the JSON config):

| Variable | Default | Purpose |
|---|---|---|
| `Z2M_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |

## What gets surfaced

- **Lights** become a single `light.*` entity per device, with `turn_on`, `turn_off`, `set_brightness` (if dimmable), `set_color_temp` (if tunable-white), and `set_color` (if RGB-capable).
- **Numeric properties** (`temperature`, `humidity`, `illuminance`, `battery`, `pressure`, `power`, `energy`, `current`) become read-only `numeric_sensor.*` entities.
- **Binary properties** (`occupancy`, `contact`, `water_leak`, `smoke`, `tamper`, `vibration`) become read-only `binary_sensor.*` entities.
- **Multi-property devices fan out**: a motion sensor exposing occupancy + temperature + humidity + battery yields four entities.

## Out of scope (v0.1)

- Z2M network management (pairing, removal, OTA updates, name changes from gohome). Use the Z2M dashboard or its own MQTT API directly.
- Action sensors (`action: "single"`, etc.) — these are events, not state.
- Climate, cover, lock, fan device classes (no proto support yet).
- Switch / smart-plug actuators (writable `state` properties). Smart plugs that also expose `power`/`energy` will surface those read-only entities; the writable `state` is logged once at INFO and skipped.

## Known caveats

- New devices paired in Z2M show up automatically — no driver restart needed.
- If Z2M is configured to publish state non-retained (recent default), entity state stays at the mapper-assigned defaults until the device's next state change. Toggling the device once seeds it; subsequent state is live.
- Per-device `availability` requires Z2M's availability feature to be enabled server-side. Without it, entities default to `Available=true` and can drift if a battery device dies — Z2M's `bridge/devices` topic doesn't carry liveness on its own.
- A successful publish to `<base>/<friendly>/set` is reported as `ok=true`. If Z2M silently ignores the command (invalid friendly_name, device unreachable, etc.), gohome won't know — there's no MQTT 5 request/response in v0.1.
- A device that adds a property after pairing (firmware update) is not picked up until the driver restarts.

## Source

[`drivers/z2m/`](.) in the gohome monorepo.
