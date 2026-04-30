package listener

import (
	"net/http"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func newH2CServer(h http.Handler) http.Handler {
	return h2c.NewHandler(h, &http2.Server{})
}
