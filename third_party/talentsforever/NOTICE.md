# talentsforever.com data notice

`data.json` is an unmodified response body retrieved from
[talentsforever.com/data.json](https://talentsforever.com/data.json). Its exact
retrieval time, publisher date, HTTP metadata, byte count, and integrity hashes
are recorded in `manifest.json`.

The publisher declares the export to be licensed under
[CC BY 4.0](https://creativecommons.org/licenses/by/4.0/) and supplies this
attribution:

> Data from talentsforever.com (https://talentsforever.com)

This dataset is not covered by the repository's MIT license. The export also
states that game names, icons, and tooltip text belong to Blizzard
Entertainment.

The export is a fan-made transcription of demo footage and Blizzard slides. It
contains confirmed observations, estimates, Classic fallbacks, and editorial
material. It can lag the game and is not simulator input. No production package
loads this directory, and updating the snapshot does not activate any mechanic.

From the repository root:

```sh
make forever-data-verify
make forever-data-check
make forever-data-update
```

`verify` is offline. `check` and `update` fetch the source; `check` never writes,
and `update` validates the complete response before replacing the rolling
snapshot. `check` normally exits successfully even when it reports a change;
pass `--fail-on-change` directly to the tool when a non-zero automation signal
is useful. Git history retains earlier snapshots.

After an intentional manifest schema or canonicalizer change, run
`make forever-data-rebuild-manifest`. This offline migration preserves the
original snapshot and retrieval metadata, and rewrites only the derived
manifest.
