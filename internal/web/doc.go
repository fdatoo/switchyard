// Package web serves the embedded Switchyard app and widget assets.
//
// It owns HTTP fallback routing for the single-page app, immutable cache
// headers for built assets, lightweight health checks, and widget-pack file
// serving. API traffic is handled by the Connect listener, not this package.
package web
