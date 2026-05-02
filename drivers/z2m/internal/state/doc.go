// Package state translates between Zigbee2MQTT device descriptors and
// switchyard entityv1.Attributes. Pure functions, no I/O. The exported
// surface is small: EntityID, EntitiesFor, MergeState, Reconcile,
// CommandToPayload.
package state
