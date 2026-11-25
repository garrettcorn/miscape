# Roadmap (deferred from v0.1, prioritized)

## P0: before any public official world
1. **Real-device performance pass.** Test on a low-end Android, an iPhone
   (Safari) and an integrated-GPU laptop. Tune the Low preset. Test Firefox
   and Safari.
2. **Real accounts** for official worlds (OAuth / passkeys). Replace bearer
   character tokens there, and add rate-limited login and session revocation.
3. **Load test** an official shard (64 bots). Move snapshots to a compact
   binary encoding if bandwidth exceeds budget.
4. **Authenticated ops API** for official worlds (moderation, bans, broadcast).
5. **Gzip/brotli static assets** and cache-busting filenames.
6. **Graceful shutdown on Windows service stop** (Ctrl-C works; service-manager
   stop events still need handling).

## P1: player-hosted polish
7. **Browser-native joining** once Tailcat's js/wasm build is stable:
   relay-only at first (no companion needed), direct paths when Tailcat ships
   WebRTC (tailcat#4).
8. Tunnel-level allowlist (`AllowClient` with per-friend client keys) as an
   optional extra layer beyond invite codes.
9. One-click packaged builds (Windows/macOS/Linux) with an "Invite" button in
   the host UI. Companion remembers the last world. Keep the browser origin
   stable so characters stay signed in.
10. Optional hosted relay service for players who can't run the companion
    (it costs money and sees encrypted traffic metadata, so it needs a privacy
    review).
11. Scheduled off-machine backups and a restore picker in the admin screen.

## P2: content and gameplay
12. A third zone and a Gloam boss with a telegraphed attack. More recipes and
    equipment slots (armor, offhand lantern as an item slot).
13. Ranged combat, dodge, and better creature pathfinding (A* on the voxel grid).
14. A bank or storage chest, and housing plots with per-plot permissions.
15. Day/night cycle, weather, audio and music, particle effects (with the
    reduce-effects setting respected).
16. Server-side movement replay (input-based prediction and reconciliation)
    instead of envelope checks.

## P3: platform
17. Postgres storage for official worlds (row-level transactions per trade),
    and horizontal sharding with a matchmaking/lobby service.
18. Mod API 2: custom blocks and textures (validated atlases), mod
    dependencies, signed mod bundles. A sandboxed scripting layer
    (WASM, capability-limited, metered) only after a security review.
19. Localization and screen-reader-friendly menus.
