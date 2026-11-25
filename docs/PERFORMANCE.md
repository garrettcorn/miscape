# Performance: budgets, measurements, device classes

## Budgets (targets for v0.1)

| Metric | Budget | How it's enforced |
|---|---|---|
| Frame time (CPU+GPU) | ≤ 16.6 ms on the minimum targets at **Low**; ≤ 8 ms typical desktop at High | Chunked meshes, per-chunk frustum culling, LOD, instanced entities |
| Draw calls | ≤ 120 / frame | One draw per visible chunk mesh (+ water), one instanced draw for every entity, sky, highlight |
| Triangles | ≤ 60k (High), ≤ 25k (Low) | Face culling, AO quad flipping, 2× LOD beyond `lodDist` |
| Meshing | ≤ 3 ms (Low) / 5 ms per frame | Time-sliced mesh queue, nearest chunks first |
| GPU memory | ≤ 16 MB | 8-byte vertices, shared index buffer, LOD meshes dropped with hysteresis |
| JS heap | ≤ 32 MB | Fixed world array (640 KB), reused buffers |
| Initial download | ≤ 100 KB gzipped, no images | Procedural texture atlas, no frameworks |
| Network | ≤ 16 KB/s down, ≤ 2 KB/s up per player | 10 Hz interest-managed snapshots, 20 Hz move updates, RLE chunks on demand |

**Minimum targets (not yet verified on real hardware, see below):** a 2018
mid-range Android phone (Adreno 5xx / Mali-G7x class) on Chrome; an iPhone 8 /
A11 on Safari 15+; a 2015 laptop with Intel HD 520 integrated graphics. All
need WebGL2.

## Measured (2026-11-25)

**Method.** In-game, `draw()` + `gl.finish()` timed synchronously over a
240-frame scripted orbit of the village at 1280×720 CSS pixels, per quality
preset. This measures the real CPU+GPU cost of a frame, not vsync-limited fps.
Network was read from the client's byte counters (JSON payload sizes before
WebSocket compression). Asset sizes come from the repository files. Use
`?bench=1` or **Settings → Show performance overlay** to reproduce.

**Machine:** Windows 11, Chromium 156 (Claude desktop browser pane),
ANGLE/D3D11, **NVIDIA RTX 2080 SUPER**, 12 threads, DPR 1. This is a
high-end desktop.

| Preset | Render res | Frame avg | p95 | max | Draws | Triangles | Visible / meshed chunks | GPU est. | JS heap | Mesh all chunks |
|---|---|---|---|---|---|---|---|---|---|---|
| Low | 832×468 | 0.20 ms | 0.5 ms | 3.9 ms | 23 | 16.8k | 19 / 83 | 2.7 MB | 5.6 MB | 85 ms total |
| Medium | 1088×612 | 0.11 ms | 0.2 ms | 0.5 ms | 43 | 32.4k | 37 / 104 | 3.2 MB | 7.0 MB | 121 ms total |
| High | 1280×720 | 0.13 ms | 0.2 ms | 3.8 ms | 48 | 54.9k | 41 / 72 | 3.2 MB | 8.1 MB | 136 ms total |

(Full meshing of every chunk costs about 1–1.5 ms per chunk. At runtime it's
spread across frames with a 3–5 ms budget.)

| Other measurement | Value |
|---|---|
| Client download | 143 KB raw / **44 KB gzipped** (HTML+CSS+JS, no images) |
| Whole world chunk data | 55 KB RLE (640 KB raw) |
| Server binary | 31 MB (includes Tailcat/WireGuard stack) |
| Network, 1 other player + 7 creatures in view | ~10 KB/s down (idle or moving), ~1.1 KB/s up while moving |
| Tailcat join (same machine) | tunnel up in ~0.2 s via DERP, then upgraded to direct path |

## Device / browser classes tested

| Class | Tested? | Notes |
|---|---|---|
| High-end Windows desktop, Chromium (WebGL2/ANGLE D3D11) | ✅ Played end to end: join, quest, gather, craft, combat, Gloam, trade between two sessions, perf benchmark | |
| Phone-sized viewport (375×812, touch UI forced) in the same Chromium | ✅ Layout, touch controls, panels, join screen | **Emulated viewport only, not a real phone GPU or touch hardware** |
| Gamepad | ✅ Standard-mapping input path verified with a mocked `navigator.getGamepads()` | No physical controller was attached |
| Real low-end Android / iPhone / integrated-GPU laptop | ❌ **Not yet tested** | Highest-priority gap; see ROADMAP P0 |
| Firefox, Safari | ❌ Not yet tested | Uses only standard WebGL2 + ES modules, but unverified |

The frame numbers above show plenty of headroom on desktop GPUs. They are
**not** evidence that the minimum targets are met. The *Auto* preset picks
Low/Medium on low-memory and mobile devices as a precaution until real-device
runs are done.
