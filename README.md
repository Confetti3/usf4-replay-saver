# USF4 Replay Saver

Ultra Street Fighter IV on PC keeps only your last 10 matches as replays. The game cycles through 10 slots and overwrites one after every match, so after a long session most of your games are gone unless you saved each one by hand in the Replay Channel.

This tool copies every one of those replays into a folder before the game overwrites it. It is a stopgap until Ember Netplay can save replays itself.

## Use it

1. Download `usf4-replay-saver-windows-amd64.zip` from the [latest release](https://github.com/Confetti3/usf4-replay-saver/releases/latest) and unzip it anywhere.
2. Double-click `usf4-replay-saver.exe` before you start playing, and leave its window open.
3. After each match it prints the replay it saved. They go to `Documents\USF4 Replays`.

It finds the game's save folder on its own and copies your current 10 replays right away, so running it after a session still rescues the last 10 matches.

Windows may warn that the app is unrecognized, because the exe is not code signed. Choose **More info** and then **Run anyway**. The source is all in this repository if you would rather build it yourself.

The tool only reads the game's files. It never changes them unless you use `-restore` (below).

## Watching a saved replay again

This part is experimental and has not been tested in the game yet.

Close the game, then run:

```
usf4-replay-saver.exe -restore "Documents\USF4 Replays\2026-10-03_19-45-35_0cd3ae36.usf4replay"
```

It puts that replay back among the game's 10 recent matches, in place of the oldest one (which it copies first, so nothing is lost). Start the game and find it with your recent replays. Your next match may overwrite it again, so save it in the game if you want it to stay there.

Steam uploads the changed save the next time you start the game. If Steam ever shows a Cloud conflict for USF4 after a restore, keep the local files.

## Options

| Option | What it does |
| --- | --- |
| `-once` | Copy what is there now and exit, instead of watching |
| `-all` | Also copy the replays you saved by hand in the game (up to 300) |
| `-list` | List the replays already copied and check that none are damaged |
| `-out <folder>` | Copy into a different folder |
| `-save-dir <folder>` | Use this save folder instead of searching for it |
| `-restore <file>` | Put a copied replay back into the game's recent matches (experimental) |
| `-interval 5s` | How often to check for a new replay while watching |

## Linux and Steam Deck

Download the `linux-amd64` archive and run `./usf4-replay-saver` in a terminal. It looks in the usual Steam folders (`~/.steam/steam`, `~/.local/share/Steam` and the Flatpak one) and saves to `~/USF4 Replays`. Replays are stored by Steam itself, not inside the Proton prefix, so it works the same under Proton.

## Good to know

- If the tool never saves anything, check that replay recording is turned on in the game's options. It has not been confirmed yet whether the game records its 10 recent replays with that option off.
- Each file is the game's own replay data, unchanged. The name is the date and time the match was recorded plus a short checksum, which the tool uses to skip replays it already has.
- If more than one Steam account on the PC has played the game, it copies replays from all of them into the same folder.

## How the game stores replays

The game keeps its saves in Steam Cloud, in `Steam\userdata\<id>\45760\remote\CAPCOM\SUPERSTREETFIGHTERIV\SSF4_SaveData`. Every save is a numbered file with a small `N.0` file beside it that holds the CRC-32 of the save.

- Files `300` to `309` hold the last 10 matches. The game cycles through them, rewriting one after each match.
- Files `0` to `299` hold replays you saved by hand from the Replay Channel. A file called `LIST` indexes them.
- Every replay starts with `#BRP` and carries the time it was recorded at byte 16.

## Building

Install Go 1.26 or newer and run `go build`. `build.ps1` makes the release archives.

## License

MIT. See [LICENSE](LICENSE).
