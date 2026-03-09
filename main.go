package main

import (
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

func main() {
	cfg := MustLoadConfig()

	if err := os.MkdirAll(cfg.ScreenshotDir, 0755); err != nil {
		log.Fatalf("failed to create screenshot dir: %v", err)
	}

	SendTelegramMessage(cfg, "citasi started. Monitoring for appointments...")

	tracker := NewStateTracker()
	consecutiveWAF := 0

	var page *rod.Page
	var cleanup func()

	// killChrome forcefully kills any lingering Chrome processes using our profile.
	killChrome := func() {
		_ = exec.Command("pkill", "-f", "citasi-chrome-profile").Run()
		time.Sleep(2 * time.Second)
	}

	// launchNewBrowser creates a fresh browser instance.
	launchNewBrowser := func() {
		if cleanup != nil {
			// Try graceful close, ignore errors if connection is dead
			func() {
				defer func() { recover() }()
				cleanup()
			}()
			cleanup = nil
		}
		// Kill any lingering Chrome to avoid "Opening in existing browser session"
		killChrome()
		_, page, cleanup = LaunchBrowser(cfg)
	}

	launchNewBrowser()
	defer func() {
		if cleanup != nil {
			cleanup()
		}
	}()

	// browserAlive checks if the CDP connection to Chrome is still working.
	browserAlive := func() bool {
		err := rod.Try(func() {
			page.MustEval(`() => document.title`)
		})
		return err == nil
	}

	for cycle := 1; cfg.MaxCycles == 0 || cycle <= cfg.MaxCycles; cycle++ {
		log.Printf("--- Cycle %d ---", cycle)

		if !browserAlive() {
			log.Printf("Browser connection lost, restarting...")
			launchNewBrowser()
		}

		result, err := RunSingleCheck(page, cfg)
		if err != nil {
			log.Printf("Cycle %d error: %v", cycle, err)
			takeErrorScreenshot(page, cfg, cycle)

			if strings.Contains(err.Error(), "WAF") {
				consecutiveWAF++
				// Exponential backoff: 15-20 min base, doubled each consecutive hit, capped at 60 min.
				baseSeconds := 900 + rand.IntN(300) // 15-20 min
				multiplier := 1 << min(consecutiveWAF-1, 2) // 1x, 2x, 4x (cap at 4x = ~60-80 min)
				backoff := time.Duration(baseSeconds*multiplier) * time.Second
				log.Printf("WAF block detected (consecutive: %d), backing off...", consecutiveWAF)
				SendTelegramMessage(cfg, fmt.Sprintf("WAF blocked (%d in a row). Backing off for %s.", consecutiveWAF, backoff.Round(time.Second)))
				log.Printf("Next check in %s", backoff)
				time.Sleep(backoff)
				launchNewBrowser()
				continue
			}

			if strings.Contains(err.Error(), "server error") {
				// Server-side 500: transient, no browser restart needed.
				log.Printf("ICP server error (HTTP 500), will retry next cycle")
			} else {
				// Other errors: restart browser and use normal interval
				launchNewBrowser()
			}
			SendTelegramMessage(cfg, "Cycle error: "+err.Error())
		} else {
			consecutiveWAF = 0
			if result.Available && tracker.ShouldNotify(true) {
				log.Printf("SLOTS AVAILABLE!")
				SendTelegramNotification(cfg, result)
				PlayAlertSound()
			} else {
				tracker.ShouldNotify(false)
				log.Printf("No availability")
				SendTelegramMessage(cfg, "No citas disponibles.")
			}
		}

		jitter := time.Duration(rand.IntN(cfg.CheckJitter+1)) * time.Second
		sleep := time.Duration(cfg.CheckInterval)*time.Second + jitter
		// Enforce minimum 3-minute interval to avoid WAF rate limits
		minInterval := 180 * time.Second
		if sleep < minInterval {
			sleep = minInterval
		}
		log.Printf("Next check in %s", sleep)
		time.Sleep(sleep)
	}

	log.Printf("Completed %d cycles", cfg.MaxCycles)
	SendTelegramMessage(cfg, "citasi completed all cycles.")
}
