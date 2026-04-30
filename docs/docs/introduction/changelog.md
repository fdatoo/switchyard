# Changelog

gohome is in early development. This changelog tracks released versions. The project follows [Semantic Versioning](https://semver.org/); until v1.0.0 is tagged, minor versions may include breaking changes to config schema or API surfaces.

---

## v0.1.0 — 2026-04-27 (alpha)

!!! status-alpha "Alpha release"
    This is the initial public alpha. Core infrastructure is functional; APIs and config schema are not yet stable.

**Initial release.** Establishes the foundational architecture:

- `gohomed` daemon with SQLite-backed event store, in-memory state cache, and Connect-RPC API
- `gohome` CLI for config management and daemon interaction
- Carport driver protocol (`v1alpha1`) with local subprocess transport
- Pkl config loader with diff-based reload
- Starlark automation and script runtime
- MCP server with initial tool set
- Embedded web UI (React PWA)
- First-party drivers: MQTT, Zigbee2MQTT bridge, Hue, ESPHome native, generic REST, generic webhook
- Multi-user auth with passkeys, API tokens, and Pkl-declared policies
- `gohome import-ha` — Home Assistant config importer

**Known limitations in this release:**

- `gohome-edge` (remote edge agent) is not yet shipped
- Matter and HomeKit bridge drivers are in progress
- No schema migrations between alpha versions — breaking config changes may require manual updates
- MCP tool surface is incomplete; additional tools will be added in subsequent releases

---

*This file is updated with each release. For pre-release development notes, see the repository commit log.*
