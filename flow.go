package main

import (
	"fmt"
	"log"
	"math/rand/v2"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
)

type CheckResult struct {
	Available  bool
	Details    string
	Screenshot string // file path, empty if disabled
}

const noAvailabilityText = "En este momento no hay citas disponibles"

// getBaseURL returns the province-specific ICP URL.
func getBaseURL(province string) string {
	switch province {
	case "8": // Barcelona
		return fmt.Sprintf("https://icp.administracionelectronica.gob.es/icpplustieb/citar?p=%s&locale=es", province)
	case "28": // Madrid
		return fmt.Sprintf("https://icp.administracionelectronica.gob.es/icpplustiem/citar?p=%s&locale=es", province)
	default:
		return fmt.Sprintf("https://icp.administracionelectronica.gob.es/icpplus/citar?p=%s&locale=es", province)
	}
}

func randomDelay() {
	ms := 1500 + rand.IntN(2500) // 1.5-4 seconds
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

func longRandomDelay() {
	ms := 4000 + rand.IntN(4000) // 4-8 seconds
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

// humanType clicks on an element, clears it, and types text character by character
// using real keyboard events (dispatchKeyEvent). This generates proper keydown/keyup
// events that F5 WAF telemetry can observe, unlike InsertText which skips them.
func humanType(page *rod.Page, selector string, text string, t time.Duration) error {
	return rod.Try(func() {
		el := page.Timeout(t).MustElement(selector)
		el.MustClick() // moves mouse + clicks, generates focus
		time.Sleep(time.Duration(200+rand.IntN(300)) * time.Millisecond)

		// Select all + delete using KeyActions (Ctrl+A, then Backspace)
		page.KeyActions().Press(input.ControlLeft).Type(input.KeyA).MustDo()
		time.Sleep(time.Duration(100+rand.IntN(200)) * time.Millisecond)
		page.Keyboard.MustType(input.Backspace)
		time.Sleep(time.Duration(200+rand.IntN(300)) * time.Millisecond)

		// Type each character with random inter-key delay
		for _, ch := range text {
			page.Keyboard.MustType(input.Key(ch))
			time.Sleep(time.Duration(50+rand.IntN(150)) * time.Millisecond)
		}
	})
}

// stepTimeout returns the per-step timeout duration from config.
func stepTimeout(cfg *Config) time.Duration {
	return time.Duration(cfg.PageTimeout) * time.Second
}

// isWAFBlocked checks if the current page is a WAF rejection page.
func isWAFBlocked(page *rod.Page) bool {
	title := ""
	if err := rod.Try(func() {
		title = page.MustEval(`() => document.title`).String()
	}); err != nil {
		return false
	}
	return strings.Contains(title, "Request Rejected")
}

// RunSingleCheck executes the full form flow and checks availability.
func RunSingleCheck(page *rod.Page, cfg *Config) (*CheckResult, error) {
	result := &CheckResult{}
	t := stepTimeout(cfg)

	// Step 1: Navigate to the ICP page
	url := getBaseURL(cfg.Province)
	log.Printf("[flow] navigating to %s", url)
	if err := rod.Try(func() {
		page.Timeout(t).MustNavigate(url).MustWaitLoad()
	}); err != nil {
		return nil, fmt.Errorf("navigate: %w", err)
	}
	// Wait for F5 WAF JavaScript challenge to execute and set cookies.
	time.Sleep(time.Duration(5000+rand.IntN(3000)) * time.Millisecond)
	randomDelay()

	// Accept cookies if the banner is present
	if has, _, _ := page.Has("#cookie_action_close_header"); has {
		log.Printf("[flow] accepting cookies")
		_ = rod.Try(func() {
			page.Timeout(5 * time.Second).MustElement("#cookie_action_close_header").MustClick()
		})
		time.Sleep(1 * time.Second)
	}

	// Step 2a: Select oficina if configured
	if cfg.OficinaText != "" {
		log.Printf("[flow] selecting oficina: %s", cfg.OficinaText)
		if err := rod.Try(func() {
			page.Timeout(t).MustElement(SelOfficeInitial).MustSelect(cfg.OficinaText)
		}); err != nil {
			return nil, fmt.Errorf("select oficina: %w", err)
		}
		longRandomDelay()
	}

	// Step 2b: Select tramite on the initial page
	log.Printf("[flow] selecting tramite: %s", cfg.TramiteText)
	if err := rod.Try(func() {
		page.Timeout(t).MustElement(SelTramiteSelect).MustSelect(cfg.TramiteText)
	}); err != nil {
		return nil, fmt.Errorf("select tramite: %w", err)
	}
	longRandomDelay()

	// Step 3: Click Aceptar (calls envia() JS function)
	log.Printf("[flow] clicking Aceptar")
	if err := rod.Try(func() {
		page.Timeout(t).MustElement(SelBtnAceptar).MustClick()
	}); err != nil {
		return nil, fmt.Errorf("click aceptar: %w", err)
	}

	// Wait for the info page to load (it navigates to a new URL)
	log.Printf("[flow] waiting for info page...")
	if err := rod.Try(func() {
		page.Timeout(t).MustElement(SelBtnEntrar)
	}); err != nil {
		if isWAFBlocked(page) {
			return nil, fmt.Errorf("WAF blocked: the site rejected our request (rate limited)")
		}
		return nil, fmt.Errorf("wait for info page: %w", err)
	}
	longRandomDelay()

	// Step 4: Click Entrar on the info/disclaimer page
	log.Printf("[flow] clicking Entrar (info page)")
	if err := rod.Try(func() {
		page.Timeout(t).MustElement(SelBtnEntrar).MustClick()
	}); err != nil {
		return nil, fmt.Errorf("click entrar: %w", err)
	}

	// Wait for the personal data form to load.
	// Different tramites show different form variants — wait for doc number field
	// which is present in all variants.
	log.Printf("[flow] waiting for personal data form...")
	if err := rod.Try(func() {
		page.Timeout(t).MustElement(SelDocNumber)
	}); err != nil {
		if isWAFBlocked(page) {
			return nil, fmt.Errorf("WAF blocked after entrar")
		}
		return nil, fmt.Errorf("wait for personal data form: %w", err)
	}
	longRandomDelay()

	// Step 5: Fill personal data using keyboard events
	log.Printf("[flow] filling personal data")

	// Select country (only if the selector exists)
	if hasCountry, _, _ := page.Has(SelCountry); hasCountry {
		log.Printf("[flow] selecting country: %s", cfg.Country)
		if err := rod.Try(func() {
			page.Timeout(t).MustElement(SelCountry).MustSelect(cfg.Country)
		}); err != nil {
			return nil, fmt.Errorf("select country: %w", err)
		}
		randomDelay()
	}

	// Click doc type radio (only if it exists)
	docSel := docTypeSelector(cfg.DocType)
	if hasDocType, _, _ := page.Has(docSel); hasDocType {
		log.Printf("[flow] selecting doc type: %s", cfg.DocType)
		if err := rod.Try(func() {
			page.Timeout(t).MustElement(docSel).MustClick()
		}); err != nil {
			return nil, fmt.Errorf("click doc type: %w", err)
		}
		randomDelay()
	}

	// Fill doc number with real keyboard events
	log.Printf("[flow] typing doc number")
	if err := humanType(page, SelDocNumber, cfg.DocNumber, t); err != nil {
		return nil, fmt.Errorf("input doc number: %w", err)
	}
	randomDelay()

	// Fill name with real keyboard events
	log.Printf("[flow] typing name")
	if err := humanType(page, SelName, cfg.FullName, t); err != nil {
		return nil, fmt.Errorf("input name: %w", err)
	}
	longRandomDelay()

	// Submit the form — try Enviar first, then Aceptar as fallback
	log.Printf("[flow] submitting personal data form")
	if hasEnviar, _, _ := page.Has(SelBtnEnviar); hasEnviar {
		if err := rod.Try(func() {
			page.Timeout(t).MustElement(SelBtnEnviar).MustClick()
		}); err != nil {
			return nil, fmt.Errorf("click enviar: %w", err)
		}
	} else if err := rod.Try(func() {
		page.Timeout(t).MustElement(SelBtnAceptar).MustClick()
	}); err != nil {
		return nil, fmt.Errorf("click submit button: %w", err)
	}

	// Wait for next page
	time.Sleep(time.Duration(5000+rand.IntN(3000)) * time.Millisecond)

	if isWAFBlocked(page) {
		return nil, fmt.Errorf("WAF blocked after personal data submit")
	}

	// Read page text — the site may go directly to availability result
	log.Printf("[flow] checking page after submit...")
	bodyText := ""
	if err := rod.Try(func() {
		bodyText = page.Timeout(t).MustElement("body").MustText()
	}); err != nil {
		return nil, fmt.Errorf("read body text: %w", err)
	}

	// If availability result already shown, return immediately — no further steps needed
	if containsNoAvailability(bodyText) {
		log.Printf("[flow] no availability (direct result)")
		result.Available = false
		result.Details = "No availability detected"
		takeCheckScreenshot(page, cfg, result)
		return result, nil
	}

	// Step 6: Check for CAPTCHA
	if hasCaptcha(page) {
		log.Printf("[flow] CAPTCHA detected — waiting for manual solve")
		PlayAlertSound()
		waitForCaptchaSolve(page)
		log.Printf("[flow] CAPTCHA appears solved, continuing")
		longRandomDelay()
	}

	// Step 7: Select office if a second office selector appears
	if hasOffice, _, _ := page.Has(SelOfficeLater); hasOffice {
		log.Printf("[flow] selecting office")
		if err := rod.Try(func() {
			sel := page.Timeout(t).MustElement(SelOfficeLater)
			opts := sel.MustElements("option")
			for _, opt := range opts {
				val := opt.MustProperty("value").String()
				if val != "" && val != "0" {
					opt.MustClick()
					break
				}
			}
		}); err != nil {
			return nil, fmt.Errorf("select office: %w", err)
		}
		longRandomDelay()

		if hasSiguiente, _, _ := page.Has(SelBtnSiguiente); hasSiguiente {
			log.Printf("[flow] clicking Siguiente")
			if err := rod.Try(func() {
				page.Timeout(t).MustElement(SelBtnSiguiente).MustClick()
			}); err != nil {
				return nil, fmt.Errorf("click siguiente: %w", err)
			}
			time.Sleep(5 * time.Second)
		}
	}

	// Step 8: Fill contact info if present (using keyboard events)
	if cfg.Phone != "" {
		if hasPhone, _, _ := page.Has(SelPhone); hasPhone {
			log.Printf("[flow] filling phone")
			_ = humanType(page, SelPhone, cfg.Phone, t)
			randomDelay()
		}
	}

	if cfg.Email != "" {
		if hasEmail, _, _ := page.Has(SelEmail1); hasEmail {
			log.Printf("[flow] filling email")
			_ = humanType(page, SelEmail1, cfg.Email, t)
			randomDelay()
		}
		if hasEmail2, _, _ := page.Has(SelEmail2); hasEmail2 {
			_ = humanType(page, SelEmail2, cfg.Email, t)
			randomDelay()
		}
	}

	// Submit contact form if there's a button to click
	for _, sel := range []string{SelBtnEnviar, SelBtnSiguiente} {
		if has, _, _ := page.Has(sel); has {
			log.Printf("[flow] clicking submit button")
			_ = rod.Try(func() { page.Timeout(t).MustElement(sel).MustClick() })
			time.Sleep(5 * time.Second)
			break
		}
	}

	// Re-read page text for final availability check
	log.Printf("[flow] checking availability")
	if err := rod.Try(func() {
		bodyText = page.Timeout(t).MustElement("body").MustText()
	}); err != nil {
		return nil, fmt.Errorf("read body text: %w", err)
	}

	if isWAFBlocked(page) {
		return nil, fmt.Errorf("WAF blocked at availability check")
	}

	result.Available = !containsNoAvailability(bodyText)
	if result.Available {
		result.Details = fmt.Sprintf("Slots may be available!\nPage text snippet: %s", truncate(bodyText, 500))
	} else {
		result.Details = "No availability detected"
	}

	takeCheckScreenshot(page, cfg, result)
	return result, nil
}

func docTypeSelector(docType string) string {
	switch docType {
	case "nie":
		return SelDocNIE
	case "passport":
		return SelDocPassport
	case "dni":
		return SelDocDNI
	default:
		return SelDocNIE
	}
}

func hasCaptcha(page *rod.Page) bool {
	has, _, _ := page.Has("iframe[src*='recaptcha']")
	return has
}

func waitForCaptchaSolve(page *rod.Page) {
	for i := 0; i < 150; i++ { // max 5 minutes (150 * 2s)
		time.Sleep(2 * time.Second)

		if !hasCaptcha(page) {
			return
		}
		if hasNext, _, _ := page.Has(SelOfficeLater); hasNext {
			return
		}
		if hasNext, _, _ := page.Has(SelPhone); hasNext {
			return
		}

		// Re-alert every 30 seconds
		if i > 0 && i%15 == 0 {
			PlayAlertSound()
		}
	}
	log.Printf("[flow] CAPTCHA wait timed out after 5 minutes")
}

func containsNoAvailability(text string) bool {
	return strings.Contains(text, noAvailabilityText)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func takeCheckScreenshot(page *rod.Page, cfg *Config, result *CheckResult) {
	if !cfg.SaveScreenshots {
		return
	}
	ssPath := filepath.Join(cfg.ScreenshotDir, fmt.Sprintf("check_%d.png", time.Now().Unix()))
	if err := rod.Try(func() {
		page.Timeout(10 * time.Second).MustScreenshot(ssPath)
	}); err != nil {
		log.Printf("[flow] screenshot error: %v", err)
	} else {
		result.Screenshot = ssPath
	}
}

func takeErrorScreenshot(page *rod.Page, cfg *Config, cycle int) {
	if !cfg.SaveScreenshots {
		return
	}
	t := 10 * time.Second
	ssPath := filepath.Join(cfg.ScreenshotDir, fmt.Sprintf("error_cycle%d_%d.png", cycle, time.Now().Unix()))
	if err := rod.Try(func() {
		page.Timeout(t).MustScreenshot(ssPath)
	}); err != nil {
		log.Printf("[screenshot] error: %v", err)
	} else {
		log.Printf("[screenshot] saved to %s", ssPath)
	}
}
