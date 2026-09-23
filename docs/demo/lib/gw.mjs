// TCP forwarder for the recording stack. It listens on the loopback ports the
// README names (3000 for the dashboard, 8080 for the API) inside its own
// network namespace and forwards them to the compose services. The terminal
// and browser containers join this namespace (network_mode: service:gw), so
// what they type and load is exactly `localhost:3000` and `localhost:8080`,
// and no port is ever published on the host.
import net from "node:net";

const routes = (process.env.GW_ROUTES || "3000=frontend:3000,8080=backend:8080")
  .split(",")
  .map((r) => r.trim())
  .filter(Boolean)
  .map((r) => {
    const [port, target] = r.split("=");
    const [host, tport] = target.split(":");
    return { port: Number(port), host, tport: Number(tport) };
  });

for (const { port, host, tport } of routes) {
  const server = net.createServer((client) => {
    const upstream = net.connect(tport, host);
    client.pipe(upstream).pipe(client);
    const drop = () => { client.destroy(); upstream.destroy(); };
    client.on("error", drop);
    upstream.on("error", drop);
  });
  server.listen(port, "127.0.0.1", () => console.log(`gw: 127.0.0.1:${port} -> ${host}:${tport}`));
}
