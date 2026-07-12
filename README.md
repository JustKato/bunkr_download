# Bunkr Download

Small Wails app for scraping Bunkr albums and downloading files. UI mimics
old Steam since I like old school cool.

Paste an album URL like `https://example.invalid/album-redacted`, fetch the listing, preview
images in a separate window, pick an output folder, and download with filters.

Settings live in the usual OS config spot (`~/.config/bunkrdownload/settings.json`
on Linux). Previewed images get cached under `~/.cache/bunkrdownload/media/` and
reused when you download the same file later.

## Toolchain

Toolchain runs in a distrobox because I am running bazzite, for any other OS you can just follow the default wails developer configuration.

```bash
./scripts/init.sh    # once
./scripts/build.sh
./build/bin/bunkrdownload
```

Dev loop with hot reload:

```bash
distrobox enter wails-dev -- bash -lc 'PATH=$HOME/.local/bin:$PATH wails3 dev'
```

## Layout

```
main.go       app entry, window
app.go        scraping, media URLs, preview window
download.go   batch downloads + progress events
settings.go   persisted sidebar options
media_cache.go preview/download cache on disk
frontend/     html/css/js + generated bindings
scripts/      init + build helpers for distrobox
```

Tests: `go test ./...` (run inside `wails-dev` if GTK headers aren't on the host).
