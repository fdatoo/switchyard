# Concepts

This section explains the ideas that switchyard is built on. You do not need to understand all of it before getting started — the [Installation](../installation/index.md) guide gets the daemon running without any of this background. But when something behaves unexpectedly, or when you want to write automations that go beyond the examples, these pages are the reference.

## What is in this section

### [Domain model](domain-model.md)

Every noun the system uses: drivers, driver instances, devices, entities, entity classes, areas, zones, automations, scripts, scenes, dashboards, widgets, users, roles, and policies. Includes Pkl examples for each, and a quick-reference table comparing switchyard concepts to their Home Assistant equivalents.

### [Event sourcing](event-sourcing.md)

Why every state change in switchyard is an immutable event appended to a log, what that gives you in practice (time-travel debugging, audit logs, answering "what happened at 2am?"), and what it costs (disk space grows over time). Includes a worked example of using `switchyard events` to trace an unexpected light-on at 2am.

### [Config model](config-model.md)

How configuration works: Pkl for typed, validated, git-friendly declarations; Starlark for sandboxed logic that scales from inline one-liners to `.star` files. Covers secret handling (never in Pkl source), and how config reload works without restarting unchanged driver instances.

---

These three pages reinforce each other. The domain model tells you what things are; event sourcing tells you how state changes flow; the config model tells you how you declare all of the above. Together they form the conceptual foundation for everything else in this documentation.
