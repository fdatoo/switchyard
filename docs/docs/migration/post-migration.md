# Post-migration checklist

!!! status-wip "In development"
    This feature is in active development. The `switchyard import-ha` command is not yet shipped.

After `switchyard import-ha` completes, the output directory contains a structurally complete switchyard config tree — but it is not yet ready to run. Work through this checklist before pointing `switchyardd` at the output.

The items are ordered: fix structural errors first, then resolve driver configuration, then verify logic, then cut over.

---

## Checklist

- [ ] **Read `IMPORT_REPORT.md`.**  
  Open `<out-dir>/IMPORT_REPORT.md` and read the summary section. Note the FIXME count and which integrations are fully mapped versus placeholder. This gives you the full picture before you start fixing things.

- [ ] **Run `switchyard config validate`.**  
  ```
  $ switchyard config validate --config ./my-switchyard
  ```
  Fix any Pkl type errors or schema violations before proceeding. The importer generates valid Pkl under normal conditions, but complex or unusual HA configs may produce edge cases. All errors are reported with file and line references.

- [ ] **Source `IMPORTED_SECRETS.env`, then delete it.**  
  Secret values from `secrets.yaml` are in `IMPORTED_SECRETS.env`. This file is gitignored; delete it after copying the values to your permanent secret store.  
  ```
  # Source temporarily to bootstrap the running daemon:
  set -a && source ./IMPORTED_SECRETS.env && set +a
  # Then move values to your preferred store and remove the file:
  rm ./IMPORTED_SECRETS.env
  ```
  See [Secrets](../configuration/secrets.md) for the available secret store options (`env:`, `file:`, `keyring:`).

- [ ] **Re-register passkeys and passwords for each user.**  
  Authentication credentials are not migrated. For each user listed in `IMPORT_REPORT.md`:  
  ```
  $ switchyard auth bootstrap <user-slug>
  ```
  Follow the prompts to register a passkey or set a password. The user's display name, role, and ID are already in `auth/users.pkl`; only the credential is missing.

- [ ] **Install required drivers.**  
  The `IMPORT_REPORT.md` **What to do next** section lists the `switchyard driver install` commands for every driver referenced in `drivers.pkl`. Run them:  
  ```
  $ switchyard driver install ghcr.io/switchyard/driver-mqtt:v1
  $ switchyard driver install ghcr.io/switchyard/driver-hue:v1
  # ... etc.
  ```

- [ ] **Verify each driver instance is running.**  
  After applying the config and starting `switchyardd`, check that every driver instance reports a healthy status:  
  ```
  $ switchyard driver status
  ```
  Each instance should show `running`. Instances showing `error` or `starting` need attention — check their logs with `switchyard driver logs <instance-id>`.

- [ ] **Re-enter credentials for drivers that require it.**  
  Some drivers require re-pairing or re-authorisation after migration:
  - **HomeKit** — re-pair from the switchyard dashboard once the HomeKit driver is running.
  - **Matter** — re-commission each Matter device.
  - **Nest** — complete the OAuth flow via `switchyard driver auth nest`.
  - **Hue** — verify the bridge IP in `drivers.pkl` matches your current network; press the Hue bridge button when prompted.

- [ ] **Review automations with `# FIXME` markers.**  
  Find all Starlark files that need manual attention:  
  ```
  $ grep -rn 'FIXME(' ./my-switchyard/automations/
  $ grep -rn 'FIXME(' ./my-switchyard/scripts/
  ```
  For each FIXME, read the comment, understand the original Jinja, and rewrite the placeholder in Starlark. See [Jinja to Starlark](jinja-to-starlark.md) for guidance on common patterns.

- [ ] **Test a sample of automations manually.**  
  Trigger a few automations by hand to confirm they execute correctly:  
  ```
  $ switchyard automation trigger <automation-id>
  $ switchyard automation trace <correlation-id>
  ```
  Focus on automations that had FIXME markers resolved in the previous step.

- [ ] **Verify computed entities resolve correctly.**  
  For each computed entity (template sensor / binary sensor) in `entities/computed.pkl`, check that it has a valid state:  
  ```
  $ switchyard state get <entity-id>
  ```
  A value of `None` or an error in the entity's trace indicates that its Starlark expression needs fixing.

- [ ] **Verify scenes apply correctly.**  
  Apply each imported scene and confirm the expected entities change state:  
  ```
  $ switchyard scene apply <scene-id>
  ```

- [ ] **Check dashboards render correctly.**  
  Lovelace dashboards are not migrated. Open the switchyard web UI and rebuild dashboards using the WYSIWYG editor. Start with the most-used views.

- [ ] **Set up any missing secrets.**  
  If `IMPORT_REPORT.md` lists any secrets that could not be translated (e.g., because the integration they belonged to has no mapper), add them manually to `secrets.pkl` using the appropriate `read("env:...")` reference and populate the corresponding environment variable or keyring entry.

- [ ] **Commit the config directory to git.**  
  The output directory is designed to be git-initable. The `.gitignore` already excludes `IMPORTED_SECRETS.env`:  
  ```
  $ cd ./my-switchyard
  $ git init
  $ git add .
  $ git commit -m "Initial switchyard config imported from Home Assistant"
  ```

- [ ] **Run in parallel with Home Assistant for at least one week.**  
  Before decommissioning HA, run both systems simultaneously. Let switchyard handle automations while observing that behaviour matches your expectations. Pay attention to:
  - Automations that fire when they should not (over-triggering)
  - Automations that fail to fire (under-triggering, often a FIXME not yet resolved)
  - Driver entities missing or showing stale state
  - Presence and zone automations (these depend on presence drivers being correctly set up)

- [ ] **Decommission Home Assistant.**  
  Once you are satisfied that switchyard covers your needs, stop the HA daemon, point your devices and any integrations at switchyard, and archive the HA config directory as a backup.

---

## Common issues

**`switchyard config validate` fails with a type error on a driver instance.**  
A mapper produced an incorrect field type. Open the flagged file and compare the field names and types against the driver's Pkl manifest (`switchyard driver schema <driver-name>`). Usually a string where an integer is expected, or a missing required field.

**A driver instance shows `error` status.**  
Run `switchyard driver logs <instance-id>` and read the first error line. Common causes: wrong IP address (network changed since the HA config was written), API key revoked or expired, or a required credential not yet populated from `IMPORTED_SECRETS.env`.

**An automation fires but does nothing.**  
The automation's Starlark handler likely contains an unresolved `result = None` placeholder. Search the corresponding `.star` file for `FIXME` and fix the placeholder expression.

**A computed entity always shows `None`.**  
The Starlark expression for the entity's value is either a placeholder (`result = None`) or references an entity that does not exist in switchyard yet (the driver for that entity's integration may not be running). Check `switchyard driver status` and the entity's own trace.

**A secret reference fails at `config validate`.**  
The environment variable named in `read("env:UPPER_SNAKE_CASE")` is not set in the daemon's environment. Source `IMPORTED_SECRETS.env` or add the variable to your systemd unit's `EnvironmentFile=`.
