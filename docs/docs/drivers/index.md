# Using Drivers

!!! status-alpha "Alpha — shipped, interface evolving"

Drivers are the extensibility spine of gohome. Each driver is a **separate binary** that speaks the [Carport gRPC protocol](building/index.md) over a Unix domain socket. `gohomed` spawns and supervises drivers as subprocesses; drivers register entities, emit state changes, and respond to commands. The daemon knows nothing about the device — all device-specific logic lives in the driver binary.

Because drivers are separate processes, a crashing driver does not take down the daemon, and drivers can be written in any language that can implement a gRPC server.

---

## Installing and removing drivers

Drivers are installed from gohome's driver registry:

```
gohome driver install <name>          # download and install a driver binary
gohome driver install <name>@<ver>    # pin a specific version
gohome driver remove  <name>          # remove binary and disable all instances
```

After installing, declare an instance in your Pkl config (or `drivers.toml` in v0.x) and run `gohome config apply`. The daemon picks up the new instance on the next config reload.

---

## Upgrading drivers

Drivers are versioned independently from `gohomed`. Upgrading a driver binary does not require a daemon restart:

```
gohome driver upgrade <name>          # upgrade to latest compatible version
gohome driver upgrade <name>@<ver>    # upgrade to a specific version
gohome driver list                    # see installed versions and available updates
```

After upgrading, the daemon restarts the affected driver instances automatically.

---

## Driver health and status

```
gohome driver status                  # all instances and their current states
gohome driver status <instance-id>    # detail for one instance
```

A typical `gohome driver status` output:

```
INSTANCE         DRIVER           STATE     RESTARTS  LAST SEEN
hue_main         driver.hue       running   0         2s ago
z2m_bridge       driver.zigbee2mqtt  running  2       15s ago
nest_home        driver.nest      backoff   5         32s ago
```

**States:**

| State | Meaning |
|---|---|
| `spawning` | Binary is being launched |
| `awaiting_handshake` | Waiting for the Carport handshake to complete |
| `running` | Healthy and connected |
| `failed` | Crashed or health check failed; backoff before restart |
| `backoff` | Waiting before next restart attempt |
| `quarantined` | Restart budget exhausted; requires manual `gohome driver restart` |
| `stopping` | Graceful shutdown in progress |
| `stopped` | Cleanly stopped |

To force a restart from any state:

```
gohome driver restart <instance-id>
```

---

## Driver versioning

Drivers follow their own version and release cadence, independently of `gohomed`. The Carport protocol version (`v1alpha1` in v0.x) governs compatibility: any driver that speaks `v1alpha1` works with any `gohomed` that speaks `v1alpha1`. When the protocol graduates to `v1`, both the daemon and drivers will carry explicit compatibility tables.

The `gohome driver list` command shows the protocol version each installed driver was built against:

```
gohome driver list

NAME              VERSION   PROTOCOL   STATUS
driver.hue        0.4.2     v1alpha1   installed
driver.mqtt       0.3.0     v1alpha1   installed
driver.zigbee2mqtt 0.5.1   v1alpha1   installed
```

---

## Sending commands manually

For debugging and scripting, you can invoke a capability directly from the CLI:

```
gohome command send light.living_room turn_on --arg brightness=80
gohome command send switch.garden_pump turn_off
```

This goes through the same `Dispatch` path as automations and the web UI — it is not a bypass.
