# Contributing to Lanternfall

Thanks for helping make Lanternfall better! Bug reports, ideas, mods and pull
requests are all welcome.

## Contributor License Agreement (CLA)

Before your first pull request can be merged, please sign the
[Contributor License Agreement](CLA.md). It takes one comment:

1. Open your pull request.
2. A bot comment will ask you to sign. Read [CLA.md](CLA.md), then reply on the
   pull request with exactly:

   ```
   I have read the CLA Document and I hereby sign the CLA
   ```

3. The **CLA** check turns green. You only need to sign once; it covers all
   your future contributions.

**Why a CLA?** Lanternfall is open source under the
[AGPL-3.0](LICENSE). The CLA lets the maintainer also offer commercial
licenses, which helps fund development. You keep the copyright in your work,
and the CLA promises that everything you contribute will **always remain
available as open source** (Section 4).

Every commit author in a pull request must have signed. Make sure the email
on your commits is linked to your GitHub account, or the bot can't match you.

## Changelog

Please don't edit `CHANGELOG.md` in pull requests. The maintainer writes entries at release time
following [Common Changelog](https://common-changelog.org/), and links them to your commits. Clear,
imperative commit messages ('Add trading timeout', 'Fix wisp leash') make that easy.

## Development

```bash
go test ./...
```

```bash
go run ./cmd/lanternfall serve
```

- Keep `gofmt` clean; CI runs `gofmt`, `go vet` and the tests.
- Game rules live in `internal/game` and must be validated on the server.
  Never trust values the client sends for anything valuable.
- New rules need tests. For trades, keep the item-conservation tests passing.
- The client in `web/` has no build step. Keep it dependency-free and small.
- Bump the protocol version in `internal/protocol` for breaking wire changes.

## Mods

Mods don't need to be in this repository, and the CLA doesn't apply to mods you
publish elsewhere. See [docs/MODS.md](docs/MODS.md).

## Art and content

Only submit art, text or other assets you created yourself, or that have a
license compatible with the AGPL. Say where any third-party material comes from.
Please don't submit content copied from other games.
