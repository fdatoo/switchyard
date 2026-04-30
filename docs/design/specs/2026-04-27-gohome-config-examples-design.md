# GoHome Configuration Examples Design

**Date:** 2026-04-27  
**Status:** Design  
**Scope:** Create two starter configuration files (`minimal-main.pkl` and `full-main.pkl`) with inline documentation to help new users understand how to configure gohome.

## Overview

The gohome daemon uses Pkl (a configuration language) to define the entire system: entities, automations, scripts, dashboards, authentication, and connectivity. Currently, there are no starter examples to help new users understand what a working configuration looks like or what's required vs. optional.

This design creates two complementary example files:
1. **Minimal config** — smallest functional setup, for users who want to start simple
2. **Full-featured config** — demonstrates all major subsystems with realistic smart home patterns

Both files are self-documenting with inline comments explaining structure, required fields, and common patterns.

## Minimal Example (`examples/minimal-main.pkl`)

### Purpose
Show the absolute minimum config needed to get gohome running. New users can copy this, understand each section, and customize it for their setup.

### Content Structure
```
1. Listeners (UDS + TCP defaults explained)
2. One entity (light.living_room)
3. Comment notes on: where to add more, what's required
```

### Key Features
- **~30-40 lines** including comments
- **Listeners section** with defaults for both UDS (socket) and TCP (HTTP)
- **Single entity example** (a light) demonstrating the entity pattern and required fields
- **Comments explain**: 
  - Why listeners are configured this way
  - How to add more entities
  - Where each subsystem lives (entities, automations, etc.)
  - What each field means in the entity definition
- **Tone**: Friendly, assumes user knows what a light entity is but not the config syntax

### Example Structure
```pkl
module gohome.config

import "gohome:entities" as ent
// ... other imports

driverInstances: Listing<cp.DriverInstance> = new {}
entities: Listing<ent.Entity> = new {
  new {
    id = "light.living_room"
    // ... minimal fields needed
  }
}

// Other sections empty but present with comments explaining their purpose
automations: Listing<auto.Automation> = new {}
scripts: Listing<scr.Script> = new {}
dashboards: Listing<dash.Dashboard> = new {}
users: Listing<authmod.User> = new {}
roles: Listing<authmod.Role> = new {}
policies: Listing<authmod.Policy> = new {}
mcp: mcpmod.MCPConfig = new mcpmod.MCPConfig {}

listener: Listener = new Listener {}
// ... listener config with inline comments
```

---

## Full-Featured Example (`examples/full-main.pkl`)

### Purpose
Demonstrate how all major gohome subsystems work together. Serves as a reference for:
- How entities relate to automations and dashboards
- What realistic automation patterns look like
- How authentication works
- How scripts can orchestrate complex behavior
- How MCP (Model Context Protocol) integrates

### Scenario
**A smart home setup** across two rooms:
- **Living room**: Light fixtures, switch, temperature sensor, motion sensor
- **Bedroom**: Light, switch, temperature sensor
- **Automations**: Motion-activated lights, temperature alerts, bedtime routine
- **Scripts**: Morning routine (lights + coffee machine trigger), Evening shutdown
- **Dashboard**: Overview of all devices and recent activity
- **Auth**: Two users (owner, guest), one guest role with limited access

### Content Structure

#### 1. Entities (~40 lines)
- `light.living_room_ceiling`
- `light.living_room_accent`
- `light.bedroom`
- `switch.living_room_fan`
- `sensor.temperature_living_room`
- `sensor.temperature_bedroom`
- `sensor.motion_living_room`

Each entity has comments explaining:
- What the `id` format is (type.name)
- What fields are required vs. optional
- How to customize for different device types

#### 2. Driver Instances (~20 lines)
- Show how drivers are configured (e.g., a Zigbee coordinator, MQTT broker)
- Comment: "These connect to your physical hardware"

#### 3. Automations (~50 lines)
- **Motion-activated lights**: "When motion detected in living room, turn on light (if dark)"
- **Temperature alert**: "When bedroom temp exceeds 28°C, notify user"
- **Bedtime routine**: "At 11 PM, turn off all lights and log 'good night'"

Each automation shows:
- How conditions work (triggers, conditions, actions)
- How to reference entities
- Common pattern: trigger → condition check → action

#### 4. Scripts (~30 lines)
- **Morning routine**: "Turn on lights, run coffee machine startup"
- **Evening shutdown**: "Turn off all lights, close blinds, arm security"

Each script shows:
- How to chain multiple actions
- How to reference entities from scripts
- Use case: "Run this manually or trigger from automation"

#### 5. Dashboards (~30 lines)
- One dashboard widget showing:
  - All lights (with on/off state)
  - Temperature sensors (current reading)
  - Recent automations (what ran and when)

Comments explain:
- How widgets reference entities
- How to arrange and customize display

#### 6. Authentication (~40 lines)
- **Users**: Owner (admin), Guest (limited)
- **Roles**: Admin, Viewer
- **Policies**: Admin can control all devices; Viewer can only see status

Shows patterns for:
- Creating users with credentials
- Assigning roles
- Writing access control rules

#### 7. MCP Configuration (~20 lines)
- Example: Connect to Claude's MCP for smart interactions
- Comments explain what MCP is and why you'd configure it

#### 8. Listeners (~15 lines)
- Same as minimal, but TLS example uncommented to show HTTPS setup

### Key Features
- **~150-200 lines** including comments
- **Realistic but simple**: Not overwhelming, but demonstrates all subsystems
- **Comments at three levels**:
  - **Section headers**: "## Automations — Make your home smart"
  - **Entry comments**: Explain each automation/script/entity
  - **Field comments**: Clarify required vs. optional fields, example values
- **Cross-references**: Comments point out how sections relate (e.g., "This automation triggers the light entity defined above")
- **Tone**: Helpful and encouraging; explains the "why" not just the "what"

---

## Supporting File (`examples/README.md`)

### Content
- **Heading**: "GoHome Configuration Examples"
- **Two-section layout**:
  1. **New to gohome?** → Start with `minimal-main.pkl`
  2. **Want to see everything?** → Check out `full-main.pkl`
- **Quick note**: Both files are functional; copy one and customize for your setup
- **Links**:
  - Link to main docs (in `docs/` submodule) for detailed guides
  - Link to Pkl language reference if needed

---

## Implementation Notes

### Files to Create
- `examples/minimal-main.pkl`
- `examples/full-main.pkl`
- `examples/README.md`

### Directory Structure
```
gohome/
├── examples/
│   ├── minimal-main.pkl       (new)
│   ├── full-main.pkl          (new)
│   └── README.md              (new)
```

### Pkl Imports
Both files will need the same imports as the current `internal/config/pkl/gohome/config.pkl`:
```pkl
import "gohome:entities" as ent
import "gohome:carport" as cp
import "gohome:automations" as auto
import "gohome:scripts" as scr
import "gohome:dashboards" as dash
import "gohome:auth" as authmod
import "gohome:mcp" as mcpmod
```

### Validation
- Both `.pkl` files should be syntactically valid Pkl
- They should not be executed (just examples), but comments can reference the actual types from `internal/config/pkl/`
- The README should be in Markdown and readable

### Comments Strategy
- Use `//` for inline comments (Pkl style)
- Use comment blocks (`// ...`) before sections to introduce them
- Avoid over-commenting; assume user has basic gohome familiarity
- Focus comments on "why" and "how to customize" not "what this line does"

---

## Success Criteria

1. ✅ **Minimal config** is ~30-40 lines, fully functional, and shows the bare minimum
2. ✅ **Full config** is ~150-200 lines, demonstrates all subsystems, and is realistic
3. ✅ Both files have clear, helpful inline comments
4. ✅ README guides new users to the right example
5. ✅ Files are syntactically valid Pkl
6. ✅ Both files are ready to copy and customize

---

## Future Enhancements (Out of Scope)

- Interactive config generator (would be a separate tool)
- Config validation tests (could be added later)
- Domain-specific examples (e.g., specific home automation platforms)
- Video walkthrough of configuring gohome
