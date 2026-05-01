# driver-hue

GoHome Carport driver for the [Philips Hue](https://www.philips-hue.com/) bridge.

- CLIP v2 only (Hue bridge firmware 1.48+, shipped 2021).
- One driver instance = one bridge.
- White and tunable-white control: `turn_on`, `turn_off`, `set_brightness`, `set_color_temp`.

## Quick start

### 1. Find your bridge

If you have one bridge on your network, the simplest path is the Philips discovery cloud endpoint:

```bash
curl -s https://discovery.meethue.com | jq
# [{"id":"...","internalipaddress":"192.168.1.10","port":443}]
```

Or browse to your router's DHCP table and look for a device named `Philips-hue`.

### 2. Get an API key

Press the round button on top of the bridge, then within 30 seconds:

```bash
curl -k -X POST https://192.168.1.10/api \
  -H 'Content-Type: application/json' \
  -d '{"devicetype":"gohome#hue-driver","generateclientkey":true}'
# [{"success":{"username":"<your-api-key>","clientkey":"..."}}]
```

The `username` field is your API key. Store it in your secret manager and reference it in the driver config.

### 3. Configure the driver

The driver receives its instance config as a JSON blob in the `GOHOME_CARPORT_INSTANCE_CONFIG` environment variable, which `gohomed` populates from the carport instance's `config_json` TOML field.

| Field | Type | Required | Default | Purpose |
|---|---|---|---|---|
| `bridge_address` | string | yes | — | IP or hostname of the bridge |
| `api_key_env` | string | yes | — | Name of an env var holding the API key (referenced indirectly so secrets stay out of config files) |
| `tls_skip_verify` | bool | no | `true` | The bridge ships a self-signed cert |

Example carport instance TOML:

```toml
[[carport.instances]]
id = "hue-living-room"
binary = "/usr/local/bin/hue-driver"
config_json = '''
{
  "bridge_address": "192.168.1.10",
  "api_key_env": "HUE_API_KEY"
}
'''
```

Operational env vars (independent of the JSON config):

| Variable | Default | Purpose |
|---|---|---|
| `HUE_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |

## Known caveats

- New bulbs paired after the driver starts are not picked up until the driver restarts.
- Color bulbs (RGB/XY) are listed and can be turned on/off, but color isn't controllable yet — the gohome `Light` proto doesn't carry color fields.
- Groups, scenes, motion sensors, and dimmer switches are out of scope for v0.1.
