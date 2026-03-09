# CitaSi

CLI tool that automates checking for Spanish "cita previa" (immigration appointment) availability on the ICP site and notifies via Telegram when slots open up.

## Features

- Automated form navigation through the ICP multi-step booking flow
- Telegram notifications (text + screenshot) when appointments become available
- Sound alerts for CAPTCHA solving and availability detection
- F5 BIG-IP WAF bypass (stealth browser, real keyboard events, persistent profile)
- Configurable check intervals with jitter
- Screenshot capture for each check cycle

## Requirements

- Go 1.22+
- Google Chrome installed at `/Applications/Google Chrome.app` (macOS)
- Telegram bot token (from [@BotFather](https://t.me/BotFather))

## Setup

1. Clone the repo and copy the env template:
   ```bash
   cp .env.example .env
   ```

2. Fill in `.env` with your details (see [Configuration](#configuration)).

3. Build and run:
   ```bash
   CGO_ENABLED=0 go build -o citasi .
   ./citasi
   ```

> `CGO_ENABLED=0` is required on macOS 26+ with Go 1.22 due to a `dyld` compatibility issue.

## Configuration

All configuration is via `.env` file:

| Variable | Required | Description |
|---|---|---|
| `PROVINCE` | Yes | Province code (`8` = Barcelona, `28` = Madrid) |
| `TRAMITE_TEXT` | Yes | Exact visible text of the tramite option |
| `DOC_TYPE` | Yes | `nie`, `passport`, or `dni` |
| `DOC_NUMBER` | Yes | Document number (e.g. `Y1234567X`) |
| `FULL_NAME` | Yes | Full name as on the document |
| `COUNTRY` | Yes | Nationality (exact text from the dropdown) |
| `OFICINA_TEXT` | No | Office name (empty = "Cualquier oficina") |
| `PHONE` | No | Contact phone number |
| `EMAIL` | No | Contact email |
| `TELEGRAM_TOKEN` | No | Bot API token for notifications |
| `TELEGRAM_CHAT_ID` | No | Chat/group ID for notifications |
| `CHECK_INTERVAL` | No | Seconds between checks (default: `60`, minimum enforced: `180`) |
| `CHECK_JITTER` | No | Random jitter added to interval (default: `15`) |
| `MAX_CYCLES` | No | Max check cycles, `0` = unlimited (default: `0`) |
| `PAGE_TIMEOUT` | No | Timeout per page operation in seconds (default: `30`) |
| `SAVE_SCREENSHOTS` | No | Save screenshots (default: `true`) |
| `SCREENSHOT_DIR` | No | Screenshot directory (default: `screenshots`) |

**Recommended intervals** to avoid WAF rate limiting:
```
CHECK_INTERVAL=180
CHECK_JITTER=60
```

## How It Works

1. Launches Chrome with automation flags stripped and stealth scripts injected
2. Navigates to the ICP appointment page for the configured province
3. Selects the tramite, fills personal data using real keyboard events
4. Checks if appointments are available
5. If available: sends Telegram notification with screenshot, plays sound alert
6. If not: waits for the configured interval and repeats
7. On WAF block: exponential backoff (15-20 min, doubling on consecutive blocks, capped at ~60 min)

## CAPTCHA

If the site shows a CAPTCHA, the tool will play a sound alert and wait up to 5 minutes for you to solve it manually in the browser window.

## Project Structure

```
main.go         Entry point, scheduler loop
config.go       .env loading and validation
browser.go      Chrome launch with anti-detection
flow.go         Multi-step form automation
selectors.go    CSS selectors for the ICP site
notify.go       Telegram notifications, sound alerts
flow_test.go    Unit tests
```
