// Package push implements web-push subscription storage and notification fanout.
//
// The current store is in-memory and the notifier is policy-light: callers feed
// interesting events into the package, which resolves a user's subscriptions and
// sends only events above the configured severity threshold.
package push
