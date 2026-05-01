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

The driver reads three environment variables, set by `gohomed`:

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `HUE_BRIDGE_ADDRESS` | yes | — | IP or hostname of the bridge. |
| `HUE_API_KEY` | yes | — | Application key from step 2. |
| `HUE_TLS_SKIP_VERIFY` | no | `true` | The bridge ships a self-signed cert. |

## Known caveats

- New bulbs paired after the driver starts are not picked up until the driver restarts.
- Color bulbs (RGB/XY) are listed and can be turned on/off, but color isn't controllable yet — the gohome `Light` proto doesn't carry color fields.
- Groups, scenes, motion sensors, and dimmer switches are out of scope for v0.1.
