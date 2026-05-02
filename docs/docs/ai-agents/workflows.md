# Example AI Workflows

!!! status-alpha "Alpha — shipped, interface evolving"

These three end-to-end examples show Claude working with switchyard tools to complete real home-automation tasks. Each example shows the conversation between the user and Claude, the tool calls Claude makes, and the results returned — not just the final answer.

---

## 1. Create an automation

**User:** "Create an automation that turns on the garage lights when motion is detected at night."

---

Claude starts by discovering what entities are available.

**Tool call — `switchyard__list_entities`**
```json
{
  "name": "switchyard__list_entities",
  "arguments": {
    "areas": ["garage"],
    "limit": 50
  }
}
```

**Result**
```json
{
  "entities": [
    {
      "entity_id": "binary_sensor.garage_motion",
      "name": "Garage Motion Sensor",
      "class": "binary_sensor",
      "state": "off",
      "area": "garage",
      "subscribe_uri": "switchyard://entities/binary_sensor.garage_motion"
    },
    {
      "entity_id": "light.garage",
      "name": "Garage Light",
      "class": "light",
      "state": "off",
      "area": "garage",
      "subscribe_uri": "switchyard://entities/light.garage"
    }
  ],
  "next_cursor": null
}
```

Claude has found `binary_sensor.garage_motion` and `light.garage`. Now it reads the existing automations file to understand the current structure before adding to it.

**Tool call — `switchyard__read_config_file`**
```json
{
  "name": "switchyard__read_config_file",
  "arguments": {
    "path": "automations/lights.pkl"
  }
}
```

**Result**
```json
{
  "path": "automations/lights.pkl",
  "content": "// automations/lights.pkl\nimport \"switchyard:automations\" as automations\n\nautomations: Listing<automations.Automation> = new {\n  new automations.Automation {\n    id = \"hall_motion_light\"\n    ...\n  }\n}\n",
  "size_bytes": 612,
  "sha256_hex": "a3f2c1d4..."
}
```

Claude writes the updated file with the new automation appended.

**Tool call — `switchyard__write_config_file`**
```json
{
  "name": "switchyard__write_config_file",
  "arguments": {
    "path": "automations/lights.pkl",
    "content": "// automations/lights.pkl\nimport \"switchyard:automations\" as automations\n\nautomations: Listing<automations.Automation> = new {\n  new automations.Automation {\n    id = \"hall_motion_light\"\n    triggers = new {\n      new automations.StateChangeTrigger {\n        entities = new { \"binary_sensor.hall_motion\" }\n        to = \"on\"\n      }\n    }\n    actions = new {\n      new automations.CallServiceAction {\n        entity     = \"light.hall\"\n        capability = \"turn_on\"\n      }\n    }\n  }\n  new automations.Automation {\n    id = \"garage_motion_lights_night\"\n    triggers = new {\n      new automations.StateChangeTrigger {\n        entities = new { \"binary_sensor.garage_motion\" }\n        to = \"on\"\n      }\n    }\n    conditions = new {\n      new automations.TimeCondition {\n        after  = \"22:00\"\n        before = \"06:00\"\n      }\n    }\n    actions = new {\n      new automations.CallServiceAction {\n        entity     = \"light.garage\"\n        capability = \"turn_on\"\n      }\n    }\n  }\n}\n"
  }
}
```

**Result**
```json
{
  "path": "automations/lights.pkl",
  "sha256_hex": "b7e3d2f1...",
  "size_bytes": 1104
}
```

Claude validates the updated file before applying.

**Tool call — `switchyard__validate_config`**
```json
{
  "name": "switchyard__validate_config",
  "arguments": {
    "pkl_bundle": "<base64-encoded tarball of the config directory>"
  }
}
```

**Result**
```json
{
  "valid": true,
  "diff": [
    { "path": "automations/lights.pkl", "change": "modified" }
  ],
  "errors": []
}
```

Validation passed. Claude applies the config.

**Tool call — `switchyard__apply_config`**
```json
{
  "name": "switchyard__apply_config",
  "arguments": {
    "pkl_bundle": "<base64-encoded tarball of the config directory>",
    "message": "Add garage motion lights automation (night only)"
  }
}
```

**Result**
```json
{
  "applied": true,
  "diff": [
    { "path": "automations/lights.pkl", "change": "modified" }
  ],
  "applied_at": "2026-04-27T21:15:33Z"
}
```

**Claude:** "Done. I've added an automation called `garage_motion_lights_night` that turns on `light.garage` whenever `binary_sensor.garage_motion` detects motion between 22:00 and 06:00. The config has been validated and applied to the running daemon."

---

## 2. Debug why a light didn't turn on

**User:** "The living room lights didn't turn on when I came home around 6pm. Why?"

---

Claude searches the event log for state changes around that time.

**Tool call — `switchyard__query_events`**
```json
{
  "name": "switchyard__query_events",
  "arguments": {
    "entity_prefix": "light.living_room",
    "kinds": ["state_changed"],
    "from_time": "2026-04-27T17:45:00Z",
    "to_time": "2026-04-27T18:15:00Z",
    "limit": 50
  }
}
```

**Result**
```json
{
  "events": [],
  "next_cursor": null
}
```

No state changes on `light.living_room` in that window. Claude looks at what was happening with the trigger entity.

**Tool call — `switchyard__query_events`**
```json
{
  "name": "switchyard__query_events",
  "arguments": {
    "entity_prefix": "binary_sensor.front_door",
    "from_time": "2026-04-27T17:45:00Z",
    "to_time": "2026-04-27T18:15:00Z",
    "limit": 50
  }
}
```

**Result**
```json
{
  "events": [
    {
      "cursor": 94812,
      "kind": "state_changed",
      "entity_id": "binary_sensor.front_door_contact",
      "occurred_at": "2026-04-27T17:58:44Z",
      "payload": { "from": "closed", "to": "open" }
    },
    {
      "cursor": 94815,
      "kind": "state_changed",
      "entity_id": "binary_sensor.front_door_contact",
      "occurred_at": "2026-04-27T17:58:51Z",
      "payload": { "from": "open", "to": "closed" }
    }
  ],
  "next_cursor": null
}
```

The door contact sensor did fire. Claude checks whether the automation ran.

**Tool call — `switchyard__query_events`**
```json
{
  "name": "switchyard__query_events",
  "arguments": {
    "kinds": ["automation_triggered", "automation_skipped"],
    "from_time": "2026-04-27T17:45:00Z",
    "to_time": "2026-04-27T18:15:00Z",
    "limit": 50
  }
}
```

**Result**
```json
{
  "events": [
    {
      "cursor": 94816,
      "kind": "automation_skipped",
      "occurred_at": "2026-04-27T17:58:51Z",
      "payload": {
        "automation_id": "arrival_living_room_lights",
        "reason": "condition_not_met",
        "condition": "TimeCondition",
        "detail": "current time 17:58 is not in range 18:00–23:59"
      }
    }
  ],
  "next_cursor": null
}
```

Found it. Claude confirms by reading the automation definition.

**Tool call — `switchyard__read_config_file`**
```json
{
  "name": "switchyard__read_config_file",
  "arguments": {
    "path": "automations/arrival.pkl"
  }
}
```

**Result**
```json
{
  "path": "automations/arrival.pkl",
  "content": "// automations/arrival.pkl\n...\n    conditions = new {\n      new automations.TimeCondition {\n        after  = \"18:00\"\n        before = \"23:59\"\n      }\n    }\n...\n",
  "size_bytes": 843,
  "sha256_hex": "c9d1e2..."
}
```

**Claude:** "The automation `arrival_living_room_lights` fired at 17:58 but was skipped because of its `TimeCondition`: it only runs between 18:00 and 23:59, and you arrived two minutes before that window opened. The door sensor triggered correctly — the lights just weren't supposed to turn on yet according to your config.

If you'd like to adjust the condition start time to, say, 17:30, I can update the automation for you."

---

## 3. Add a second Hue bridge

**User:** "Add a second Hue bridge for the garage."

---

Claude reads the current driver config to understand the existing Hue setup.

**Tool call — `switchyard__read_config_file`**
```json
{
  "name": "switchyard__read_config_file",
  "arguments": {
    "path": "drivers.pkl"
  }
}
```

**Result**
```json
{
  "path": "drivers.pkl",
  "content": "// drivers.pkl\nimport \"switchyard:drivers\" as drivers\nimport \"switchyard:drivers/hue\" as hue\n\ndrivers: Listing<drivers.DriverInstance> = new {\n  new hue.HueInstance {\n    id          = \"hue_main\"\n    bridge_host = \"192.168.1.50\"\n    api_key     = read(\"env:HUE_API_KEY\")\n  }\n}\n",
  "size_bytes": 319,
  "sha256_hex": "d4e5f6..."
}
```

Claude writes the updated file with the second bridge added.

**Tool call — `switchyard__write_config_file`**
```json
{
  "name": "switchyard__write_config_file",
  "arguments": {
    "path": "drivers.pkl",
    "content": "// drivers.pkl\nimport \"switchyard:drivers\" as drivers\nimport \"switchyard:drivers/hue\" as hue\n\ndrivers: Listing<drivers.DriverInstance> = new {\n  new hue.HueInstance {\n    id          = \"hue_main\"\n    bridge_host = \"192.168.1.50\"\n    api_key     = read(\"env:HUE_API_KEY\")\n  }\n  new hue.HueInstance {\n    id          = \"hue_garage\"\n    bridge_host = \"192.168.1.51\"\n    api_key     = read(\"env:HUE_GARAGE_API_KEY\")\n  }\n}\n"
  }
}
```

**Result**
```json
{
  "path": "drivers.pkl",
  "sha256_hex": "e7f8a9...",
  "size_bytes": 441
}
```

Claude validates first.

**Tool call — `switchyard__validate_config`**
```json
{
  "name": "switchyard__validate_config",
  "arguments": {
    "pkl_bundle": "<base64-encoded tarball>"
  }
}
```

**Result**
```json
{
  "valid": true,
  "diff": [
    { "path": "drivers.pkl", "change": "modified" }
  ],
  "errors": []
}
```

**Tool call — `switchyard__apply_config`**
```json
{
  "name": "switchyard__apply_config",
  "arguments": {
    "pkl_bundle": "<base64-encoded tarball>",
    "message": "Add hue_garage bridge at 192.168.1.51"
  }
}
```

**Result**
```json
{
  "applied": true,
  "diff": [
    { "path": "drivers.pkl", "change": "modified" }
  ],
  "applied_at": "2026-04-27T21:30:12Z"
}
```

**Claude:** "Done. I've added a second Hue instance `hue_garage` pointing at `192.168.1.51`. It reads its API key from the `HUE_GARAGE_API_KEY` environment variable. Set `HUE_GARAGE_API_KEY` in your daemon's environment before running `apply_config`, then the driver will connect immediately. If the key wasn't set first, restart the daemon after setting it to trigger reconnection.

You can check the connection status with:
```sh
switchyard status
```
or tail the event log to see the driver lifecycle events:
```sh
switchyard events tail
```"
