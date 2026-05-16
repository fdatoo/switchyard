// Package pklfs implements the filesystem-backed custom page store.
//
// Page definitions are Pkl files under the config directory. This package
// evaluates page source, tracks optional generated layout files, writes layout
// updates atomically, and exposes the storage backend used by the page service.
package pklfs
