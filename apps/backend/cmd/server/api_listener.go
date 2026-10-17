package main

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/gofiber/fiber/v3"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
)

// apiListenAddr is the host:port the API listener binds: the server's Host
// (HOST, see config.ListenHost) and Port (APP_PORT). See issue #757.
func apiListenAddr(server config.ServerConfig) string {
	return net.JoinHostPort(server.Host, server.Port)
}

// apiListenerStartupLine is the last line of the startup summary: the address
// the API listener binds, and the variable that sets it.
func apiListenerStartupLine(addr string) string {
	return fmt.Sprintf("API listener: %s (HOST sets the address; unset, it is 0.0.0.0 in a container and 127.0.0.1 outside one)", addr)
}

// apiListenConfig listens on IPv6 when host is an IPv6 address literal, and on
// IPv4 otherwise, which is Fiber's default network.
func apiListenConfig(host string) fiber.ListenConfig {
	network := fiber.NetworkTCP4
	if ip, err := netip.ParseAddr(host); err == nil && ip.Is6() && !ip.Is4In6() {
		network = fiber.NetworkTCP6
	}
	return fiber.ListenConfig{ListenerNetwork: network}
}
