// Command switchyardd runs the Switchyard daemon.
//
// The daemon owns process startup, subsystem wiring, signal handling, and
// shutdown ordering. Long-lived behavior is delegated to internal packages such
// as config, eventstore, registry, state, carport, automation, API, and web.
package main
