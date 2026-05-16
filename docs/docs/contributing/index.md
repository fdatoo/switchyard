# Contributing

switchyard is an open project and contributions are welcome. This page covers how to get involved, what kinds of contributions are most useful, and how the review process works.

---

## Code of conduct

This project follows the [Contributor Covenant](https://www.contributor-covenant.org/version/2/1/code_of_conduct/) code of conduct. In brief: be respectful, assume good faith, and do not harass or demean other contributors. Violations can be reported to the project maintainers.

---

## What we welcome

### Driver development

The highest-impact contribution you can make is a new driver. Every new driver makes switchyard useful to more people without any changes to the core. Use the [`github.com/fynn-labs/switchyard-driverkit`](https://github.com/fynn-labs/switchyard-driverkit) Go SDK and the Carport protocol to connect a device or service. See the [Building drivers](../drivers/building/index.md) guide to get started.

### Pkl module improvements

The entity schema lives in the switchyard Pkl modules. Useful work here includes:

- Adding new entity classes for device types not yet modelled
- Improving validation constraints on existing classes
- Fixing gaps in documentation comments

### Bug fixes

For non-trivial bugs, file a GitHub Issue first describing the problem and your proposed fix. This lets maintainers confirm the bug and align on the approach before you write code. For small, obvious fixes (typos, off-by-one errors with a clear correct answer), a PR without a prior issue is fine.

### Documentation

Documentation lives in this repository (`github.com/fynn-labs/switchyard-docs`). Inaccuracies, missing examples, unclear explanations — open a PR here.

### Config examples

New or improved configuration examples go in `switchyard/examples/`. The directory currently contains `minimal-main.pkl` and `full-main.pkl`. Examples should be commented and demonstrate a real, realistic use case.

---

## Pull request process

1. **Fork the relevant repository** — `switchyard`, `switchyard-driverkit`, or `switchyard-docs`, depending on what you are changing.
2. **Create a feature branch** — branch from `main` with a descriptive name (`feat/zigbee-driver`, `fix/event-replay-ordering`).
3. **Run tests** — `task test` must pass before opening a PR. See [Dev setup](dev-setup.md) for all available task targets.
4. **Keep ownership clear** — use the [Repository architecture](repo-architecture.md) map when choosing where code belongs.
5. **Open a PR with a clear description** — explain what the change does, why it is needed, and how you tested it. Link to the relevant issue.
6. **Address review feedback** — maintainers may request changes. Push follow-up commits to the same branch; do not force-push after review has started.

PRs will not be merged without a linked issue for non-trivial changes.

---

## Issue tracker

GitHub Issues on the relevant repository is the right place for:

- Bug reports
- Feature proposals
- Questions about intended behaviour

Before opening an issue, search existing issues to avoid duplicates. For bugs, include the switchyard version (`switchyard version`), your OS, and the minimal config or steps needed to reproduce the problem.

!!! note "No issue = no merge"
    Non-trivial PRs opened without a prior issue will be asked to link one before review proceeds.
