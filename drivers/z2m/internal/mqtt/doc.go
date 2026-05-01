// Package mqtt is a thin wrapper around eclipse/paho.mqtt.golang that
// exposes the subset of operations the Z2M driver needs: Connect,
// Subscribe, Unsubscribe, Publish, Close. Auto-reconnect is delegated
// to paho; OnConnect / OnDisconnect callbacks let main re-assert
// subscriptions and emit driver events on broker churn.
package mqtt
