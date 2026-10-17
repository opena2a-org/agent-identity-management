package siem

import (
	"net"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// requestView holds the client address and user agent of one request the way
// a handler receives them: c.IP behind a trusted proxy and c.Get hand out
// views of the server's request buffer, not copies. The server rewrites that
// buffer in place when it reads the next request, so a view kept past the
// request changes under its holder. overwrite rewrites both header values in
// place, with values of the same length, as the next request's parse does,
// and checks that the views changed, so a test built on it cannot pass
// without modelling the hazard.
type requestView struct {
	address   string
	userAgent string
	overwrite func()
}

func newRequestView(t *testing.T, address, userAgent string) requestView {
	t.Helper()
	app := fiber.New(fiber.Config{
		ProxyHeader:      fiber.HeaderXForwardedFor,
		TrustProxy:       true,
		TrustProxyConfig: fiber.TrustProxyConfig{Loopback: true},
	})
	fctx := &fasthttp.RequestCtx{}
	fctx.SetRemoteAddr(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 4321})
	fctx.Request.Header.Set(fiber.HeaderXForwardedFor, address)
	fctx.Request.Header.Set(fiber.HeaderUserAgent, userAgent)
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })

	view := requestView{address: c.IP(), userAgent: c.Get(fiber.HeaderUserAgent)}
	require.Equal(t, address, view.address)
	require.Equal(t, userAgent, view.userAgent)
	view.overwrite = func() {
		t.Helper()
		laterAddress := strings.Repeat("9", len(address))
		laterUserAgent := strings.Repeat("x", len(userAgent))
		fctx.Request.Header.Set(fiber.HeaderXForwardedFor, laterAddress)
		fctx.Request.Header.Set(fiber.HeaderUserAgent, laterUserAgent)
		require.Equal(t, laterAddress, view.address, "c.IP no longer hands out a view of the request buffer; this test no longer models the hazard")
		require.Equal(t, laterUserAgent, view.userAgent, "c.Get no longer hands out a view of the request buffer; this test no longer models the hazard")
	}
	return view
}
