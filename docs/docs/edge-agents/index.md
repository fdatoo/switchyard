# Edge Agents

!!! status-wip "In development"
    The edge agent is designed but partially implemented. The API may change.

An **edge agent** is an optional supervisor binary — `switchyard-edge` — that runs on a remote host and connects back to your primary `switchyardd` daemon over mTLS. It lets you co-locate drivers with the hardware they need to reach, while keeping all automation logic, event storage, and configuration on the primary.

```
switchyard-edge  ←——(mTLS, Carport v1alpha2)——→  switchyardd
(e.g. Raspberry Pi)                           (your primary host)
```

---

## What switchyard-edge is

`switchyard-edge` is a **slim relay**. It does three things:

1. **Spawns and supervises driver subprocesses** on the remote host — exactly as `switchyardd` does for local drivers.
2. **Bridges each driver's Carport stream** over an mTLS connection to the primary daemon. Drivers are completely unaware they are running on an edge host; they speak the same Carport gRPC protocol over a local Unix socket.
3. **Buffers state events to disk** when the connection to the primary is lost, then replays them in order on reconnect.

There is no automation engine, no event store, and no command authority on the edge. The primary always owns those. When the WAN link drops, drivers keep running and their events accumulate in a local ring buffer; when the link comes back, the primary receives those events as if the outage never happened.

---

## When to use an edge agent

Use an edge agent when:

- **Physical radio placement** requires a device to be remote — a Z-Wave USB stick near the wiring closet, a Zigbee dongle in the basement corner with good RF coverage, a Matter controller in a different room from your server.
- **Network segmentation** — you want drivers isolated on a separate VLAN or host, with only the mTLS tunnel reaching your primary.
- **A Pi with GPIO** — a Raspberry Pi wired into a relay board, irrigation controller, or custom sensor array that needs to live at the physical installation site.

You do **not** need an edge agent for drivers that communicate over IP (most cloud-connected drivers, MQTT bridges, local HTTP APIs). Edge agents are for hardware that requires a driver binary running on the same host as the physical interface.

---

## Example deployments

| Edge host | Drivers hosted |
|---|---|
| Raspberry Pi in the garage | Z-Wave USB stick driver (`zwave-garage`) |
| Raspberry Pi in the basement utility room | Zigbee dongle driver (`zigbee-basement`), Matter driver (`matter-basement`) |
| Raspberry Pi at a vacation home | Full local driver set; primary is at the main home |
| NUC in a server closet with a serial radio | Custom GPIO/serial driver (`relay-board`) |

---

## Architecture sketch

```
┌─────────────────────────────────────────┐
│  Primary host (runs switchyardd)            │
│                                         │
│  ┌────────────────────────────────────┐ │
│  │ switchyardd                            │ │
│  │  Carport host (UDS + TLS :7443)    │ │
│  │  Internal CA                       │ │
│  │  EventStore                        │ │
│  │  Edge registry                     │ │
│  └────────────────────────────────────┘ │
│  │                                      │
│  └─  local driver (e.g. Hue) via UDS    │
└─────────────────────────────────────────┘
             ▲
             │  mTLS, one connection per driver
             │  Carport v1alpha2
             ▼
┌─────────────────────────────────────────┐
│  Edge host (e.g. Raspberry Pi)          │
│                                         │
│  ┌────────────────────────────────────┐ │
│  │ switchyard-edge                        │ │
│  │  Driver supervisor                 │ │
│  │  Per-driver TLS bridge             │ │
│  │  On-disk ring buffer               │ │
│  │  Encrypted assignment cache        │ │
│  └────────────────────────────────────┘ │
│  │                   │                  │
│  └─ Z-Wave driver    └─ Matter driver   │
│     (subprocess,        (subprocess,    │
│      local UDS)          local UDS)     │
└─────────────────────────────────────────┘
```

Key properties:

- **Drivers are unchanged.** A driver running on an edge host speaks the same Carport protocol over the same local Unix socket. Driver authors do not need to know about edges.
- **One mTLS connection per driver.** The primary's Carport host treats each inbound TLS connection identically to a local subprocess — same `Driver.Run` stream, same dispatch semantics.
- **The primary always wins.** Assignment, configuration, and automation all live in Pkl on the primary. The edge is a transport convenience, not a peer.

---

## What switchyard-edge is not

- Not a backup daemon. When the WAN drops, automations stop running — events are buffered, but no commands are dispatched from the edge.
- Not a clustering mechanism. switchyard is deliberately single-primary; edge agents are remote driver hosts, not peers.
- Not required for IP-based drivers. If your driver communicates over the network, it can run on the primary host.

---

## In this section

- [Pairing](pairing.md) — how to pair an edge agent with your primary daemon using the mTLS enrollment flow
- [Resilience](resilience.md) — local event buffering, reconnection behavior, and multi-edge scenarios
