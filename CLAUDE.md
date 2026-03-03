# CLAUDE.md

## Build

```bash
CGO_ENABLED=0 go build -o citasi .
```

`CGO_ENABLED=0` is mandatory — Go 1.22.4 on macOS 26 crashes with `dyld missing LC_UUID` otherwise.

## Test

```bash
go test -v ./...
```

## Run

```bash
./citasi
```

Requires `.env` file with config (see `.env.example`). Chrome must be installed at the standard macOS path.

## Architecture

Single-package Go CLI (`package main`). No internal packages.

- `main.go` — Scheduler loop. Reuses browser between cycles, restarts on WAF block.
- `config.go` — Loads `.env` via godotenv, validates required fields.
- `browser.go` — Launches Chrome via rod with anti-detection: strips `--enable-automation` and other flags from `launcher.New()`, uses `stealth.MustPage()` for JS overrides, persistent profile at `~/.citasi-chrome-profile`.
- `flow.go` — Core form automation. Uses `page.Timeout(t).MustElement(sel)` pattern (per-step timeouts). Form text input uses `page.Keyboard.MustType(input.Key(ch))` for real keyboard events (not `Element.MustInput()` which uses `InsertText`).
- `selectors.go` — All CSS selectors as constants.
- `notify.go` — Telegram API (raw HTTP), macOS sound alerts (`afplay`), `StateTracker` for notification dedup.

## Key Conventions

- **Per-step timeouts**: `page.Timeout(t)` creates a new page copy with a deadline context. Never set a global timeout on the page.
- **DOM stability**: Use `MustWaitDOMStable()` or `MustWaitLoad()`, never `MustWaitStable()` (ICP site has persistent network activity that causes hangs).
- **Keyboard API**: `Keyboard.MustType()` exists. `MustPress`/`MustRelease` do NOT exist — use `page.KeyActions().Press().Type().MustDo()` for key combos like Ctrl+A.
- **Anti-detection**: Never delete `~/.citasi-chrome-profile`. Fresh profiles get WAF-blocked. The `stealth` package overrides `navigator.webdriver` which Chrome sets to `true` whenever CDP is connected.
- **Rate limiting**: Minimum 3 minutes between ICP checks. WAF backoff is 15-20 minutes.

## ICP Site Flow (Barcelona, Ukraine conflict card)

1. Page 1: Select oficina (`#sede`) + tramite (`#tramiteGrupo[0]`) → click `#btnAceptar`
2. Page 2: Info/disclaimer → click `#btnEntrar`
3. Page 3: Personal data (NIE pre-selected, fill `#txtIdCitado` + `#txtDesCitado`) → click `#btnAceptar`
4. Result: "En este momento no hay citas disponibles" or appointment slots

The form variant depends on the tramite — some have country selectors, multiple doc types, and additional steps (CAPTCHA, office selection, contact info). The flow handles both variants adaptively.
