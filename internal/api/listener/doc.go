// Package listener builds the daemon's Connect-RPC HTTP listeners.
//
// It owns TCP and Unix-domain socket setup, h2c routing, request interceptors,
// peer-credential extraction, health endpoints, and graceful shutdown. Service
// implementations stay outside this package; listener only adapts them to HTTP.
package listener
