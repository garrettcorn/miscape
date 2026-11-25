# Deploying an official world

Official worlds are ordinary HTTPS/WSS servers. They don't use Tailcat, so
browsers connect without a companion.

```bash
go build -o lanternfall ./cmd/lanternfall
```

```bash
LF_MODE=official LF_ADDR=127.0.0.1:8080 LF_DATA_DIR=/var/lib/lanternfall/shard1 LF_PUBLIC_URL=https://play.example.com ./lanternfall serve --mode official
```

Put a TLS reverse proxy in front, for example Caddy:

```
play.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

Caddy passes WebSocket upgrades through automatically. Run one process per
shard, each with its own `LF_DATA_DIR`. Each shard is its own realm, and
characters don't move between shards in v0.1.

**Operational notes**
- Use systemd or another supervisor with `KillSignal=SIGTERM`. On SIGTERM the
  server saves before exiting.
- Back up `LF_DATA_DIR` (snapshots are self-contained and checksummed). The
  server keeps 10 rotating backups locally as well.
- `GET /api/info` works as a health check. It reports protocol range, player
  count and realm.
- Official mode exposes **no** admin API. Ops change `world.json` and restart.
  An authenticated ops API is on the roadmap.
- The slice's authentication is a per-character bearer token. **Before running
  a public official service, add real accounts** (see ROADMAP P0).

**Capacity (estimate, not load-tested):** the simulation is O(players ×
nearby entities) at 20 Hz, and snapshots are about 10 KB/s per player
uncompressed (less on the wire with permessage-deflate). One small VM should
handle the configured 64 players per shard. A load test is a roadmap item.

## Player-hosted worlds

See [HOSTING.md](HOSTING.md). In short: `lanternfall host` on the player's
machine, and friends run `lanternfall join`. To avoid Tailscale's free shared
DERP relays, run your own `derper` and set `LF_DERP_MAP_URL` on both sides.
