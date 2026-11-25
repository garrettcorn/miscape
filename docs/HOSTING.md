# Hosting a private world

A private world runs on your computer. Your friends' characters, items and
levels are saved **in your world folder** and stay in that world. They're
completely separate from official-world characters.

## 1. Start the world

```bash
lanternfall host
```

You'll see:

```
🏮 Private world "My Lanternfall World" is running.
   Play:   http://127.0.0.1:8080/
   Admin:  http://127.0.0.1:8080/admin
   Admin token (paste into the admin screen): ********
   ✓ Tunnel ready. To invite a friend: open the Admin screen → Invites → Create…
```

- Play at the *Play* address. On your own computer you never need an invite.
- The world folder is `./data/private` by default. Change it with `LF_DATA_DIR`.
- `--keep-address` keeps the same friend address across restarts. Without it,
  every restart makes a new address and old invites stop working, which is the
  safer default.

## 2. Invite friends

1. Open the **Admin** page and paste the admin token. The token is also in
   `admin-token` inside the world folder.
2. Under **Invite friends**, choose how many uses and when the invite expires,
   then click **Create invite**.
3. Send the invite line (it starts with `LF1.`) **privately**, for example in a
   direct message. Don't post it publicly or show it on stream.

An invite line holds two secrets:
- a **game invite code**, checked by your world (single-use or limited, and
  expiring), and
- your **Tailcat address**, the key to the encrypted tunnel to your computer.

Revoking the invite code on the admin page stops new characters joining with
it. To also rotate the tunnel address, restart without `--keep-address`, or
delete `host-key.json` if you use it.

## 3. What your friend does

1. Install Lanternfall: get the `lanternfall` program, or run
   `go run ./cmd/lanternfall join` from the source.
2. Run `lanternfall join` and paste the invite when asked.
3. A browser opens at `http://127.0.0.1:8090/`. The invite code is already
   filled in. They pick a name and play. They keep the terminal window open
   while playing.
4. Next time they run `lanternfall join` with any fresh invite from you, the
   browser offers **Continue as <name>**. The browser remembers the character
   per world *and per local address*, so they should keep using the same port
   (default 8090). If port 8090 is busy, a random port is used and the browser
   won't find the saved character. Free the port, or pass `--port`.

The join program checks the connection and explains problems in plain language:

| Message | Meaning / fix |
|---|---|
| "Direct peer-to-peer connection" | Best case: NAT traversal worked. |
| "Using a relay … a little extra lag" | NAT traversal failed (strict firewalls, carrier NAT). Traffic goes through an encrypted DERP relay. It works, with typically +20–150 ms. |
| "Can't reach the Tailcat relay network" | The friend's internet, VPN or firewall blocks the relay directory (`tailcat.dev`). Try another network. |
| "The host didn't answer" | Your world is offline, or the invite is from before your last restart. Send a fresh invite. |
| "Version mismatch" | One of you runs an older Lanternfall. Both should update. |
| "That invite looks cut off / damaged" | Copy the whole line again. |

**Why a companion program instead of a link?** Browsers can't open Tailcat
tunnels yet. Tailcat's in-browser build is experimental and relay-only. So a
tiny local program carries the tunnel and your browser talks to it on your own
computer. See [ARCHITECTURE.md §6](ARCHITECTURE.md#6-tailcat-role-and-limits).

## 4. World settings

The admin page controls: name, description, max players, access (invite codes
or anyone with the address), PvP outside the village, difficulty, resource
abundance, trading on/off, quest reward multiplier, mods, kicking and banning.
Most changes apply immediately. Mod changes need a restart.

## 5. Saving, backups and recovery

- The world saves automatically every 30 s (`LF_AUTOSAVE_SEC`). It also saves
  right after trades, quest turn-ins, notable crafts and logouts, and when you
  stop the world with Ctrl-C.
- Saves are atomic: a new file is written and then swapped in, so a crash or
  power cut can't leave a half-written save. Each save carries a checksum.
- About every 5 minutes a copy goes to `backups/`, and the last 10 are kept.
- If `state.json` is damaged, the world starts from the newest good backup and
  tells you on the console and on the admin page.
- If saving fails (disk full, permissions), the world keeps running from
  memory, retries, and warns you and the players.

Restore a specific backup (stop the world first):

```bash
lanternfall backups
```

```bash
lanternfall restore state-20261125T162410.123Z.json
```

To back up a world yourself, copy the whole world folder while the world is
stopped. It contains secrets (`admin-token`, `host-key.json`), so keep copies private.

## 6. Mods

Put mod folders in `<world folder>/mods/`, tick **Allow mods**, enable the ones
you want, then restart. See [MODS.md](MODS.md). A world that has ever used mods
is marked *modded* on the join screen.
