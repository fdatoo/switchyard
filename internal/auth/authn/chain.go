package authn

import (
	"net/http"

	"github.com/fdatoo/gohome/internal/auth"
)

// RequestFromHTTP builds an auth.Request from an *http.Request.
func RequestFromHTTP(r *http.Request) auth.Request {
	return auth.Request{
		Scheme:     schemeOf(r),
		Headers:    r.Header,
		RemoteAddr: r.RemoteAddr,
		HTTP:       r,
	}
}

// RequestFromHTTPWithPeerCred builds an auth.Request from a Unix-domain socket
// request and attaches the peer credentials obtained via SO_PEERCRED.
func RequestFromHTTPWithPeerCred(r *http.Request, ucred *auth.PeerCred) auth.Request {
	req := RequestFromHTTP(r)
	req.Scheme = "uds:peercred"
	req.PeerCred = ucred
	return req
}

func schemeOf(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}
