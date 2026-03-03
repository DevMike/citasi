# RFC: citasi — Spanish Cita Previa Automation Tool

## Status
Implemented

## Summary
A Go CLI tool that automates the multi-step ICP (Immigration) appointment booking form to detect slot availability and notify via Telegram.

## Problem
Booking a Spanish immigration appointment ("cita previa") on the ICP website requires completing a multi-step form (province, tramite, personal data, CAPTCHA, office, contact info) only to discover "no availability." Users must repeat this manual process dozens of times per day.

## Solution
Automate the entire form-filling flow with a headless-capable browser (rod/Chrome DevTools Protocol), run on a configurable schedule, and send Telegram notifications when slots appear.

## Architecture

### Tech Stack
- **Go** — single binary, no runtime dependencies
- **rod** (`github.com/go-rod/rod`) — Chrome DevTools Protocol library with stealth mode
- **Telegram Bot API** — raw HTTP via `net/http`
- **godotenv** — `.env` config loading

### Flow
1. Launch headed Chrome via rod (with stealth to avoid automation detection)
2. Navigate to province-specific ICP URL
3. Fill multi-step form: tramite selection → personal data → CAPTCHA wait → office selection → contact info
4. Read page body text for "En este momento no hay citas disponibles"
5. If slots found: Telegram notification + macOS sound alert + screenshot
6. Close browser, wait interval + jitter, repeat

### Key Design Decisions
- **Fresh browser per cycle**: avoids site's 12-refresh limit and session accumulation (~1s overhead)
- **Headed mode**: required for manual CAPTCHA solving (macOS sound alert prompts user)
- **State tracking**: notifications only on unavailable→available transitions with 5-min cooldown
- **Single package**: 8 small files, no internal packages needed

## Configuration
All config via `.env` file (see `.env.example`):
- Province, tramite, document type/number, name, country, phone, email
- Telegram bot token and chat ID
- Check interval, jitter, max cycles, page timeout
- Screenshot toggle and directory

## Security Considerations
- No CAPTCHA bypass — pauses for manual solve
- No parallel request flooding — single sequential checks with jitter
- Credentials stored in `.env` (gitignored)
- No slot hoarding — detection only, no auto-booking

## Files
```
config.go      — Config struct, .env loading, validation
selectors.go   — All CSS selectors centralized
notify.go      — Telegram messaging, sound alerts, state tracking
browser.go     — Rod browser lifecycle with stealth
flow.go        — Multi-step form automation and availability check
flow_test.go   — Unit tests for parsing/detection logic
main.go        — Entry point, scheduler loop
```
