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

	// Step 1: Navigate to the ICP page with a warm-up visit.
	// The first load lets F5 WAF JavaScript set session cookies (TS*, TSPD_*).
	// Then we navigate away and back so the form POST carries valid cookies.
	url := getBaseURL(cfg.Province)
	log.Printf("[flow] warming up session...")
	if err := rod.Try(func() {
		page.Timeout(t).MustNavigate(url).MustWaitLoad()
	}); err != nil {
		return nil, fmt.Errorf("navigate (warmup): %w", err)
	}
	// Let F5 JS challenge fully execute and set cookies
	time.Sleep(time.Duration(8000+rand.IntN(4000)) * time.Millisecond)

	// Navigate away and back — this simulates a real user revisiting
	_ = rod.Try(func() { page.MustNavigate("about:blank") })
	time.Sleep(time.Duration(2000+rand.IntN(2000)) * time.Millisecond)

	log.Printf("[flow] navigating to %s", url)
	if err := rod.Try(func() {
		page.Timeout(t).MustNavigate(url).MustWaitLoad()
	}); err != nil {
		return nil, fmt.Errorf("navigate: %w", err)
	}
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
	// or show the options page with "Solicitar Cita"
	log.Printf("[flow] checking result page...")
	bodyText := ""
	if err := rod.Try(func() {
		bodyText = page.Timeout(t).MustElement("body").MustText()
	}); err != nil {
		return nil, fmt.Errorf("read body text: %w", err)
	}

	// If body text is too short, the page likely didn't load properly
	if len(strings.TrimSpace(bodyText)) < 50 {
		return nil, fmt.Errorf("page body too short after submit (possible loading issue): %q", truncate(bodyText, 100))
	}

	// Check for no-availability message (can appear directly after submit)
	if containsNoAvailability(bodyText) {
		log.Printf("[flow] no availability (direct result)")
		result.Available = false
		result.Details = "No availability detected"
		takeCheckScreenshot(page, cfg, result)
		return result, nil
	}

	// Check for CAPTCHA
	if hasCaptcha(page) {
		log.Printf("[flow] CAPTCHA detected — waiting for manual solve")
		PlayAlertSound()
		waitForCaptchaSolve(page)
		log.Printf("[flow] CAPTCHA appears solved, continuing")
		longRandomDelay()

		// Re-read after CAPTCHA solve
		if err := rod.Try(func() {
			bodyText = page.Timeout(t).MustElement("body").MustText()
		}); err != nil {
			return nil, fmt.Errorf("read body after captcha: %w", err)
		}
	}

	if isWAFBlocked(page) {
		return nil, fmt.Errorf("WAF blocked at availability check")
	}

	// Step 6: Handle the options page ("Solicitar Cita" / "Consultar Citas" / etc.)
	// This page appears after successful personal data submission.
	// We must click "Solicitar Cita" to proceed to actual availability.
	if isOptionsPage(page, bodyText) {
		log.Printf("[flow] options page detected — clicking 'Solicitar Cita'")
		takeCheckScreenshot(page, cfg, result) // screenshot the options page for debugging
		longRandomDelay()

		if err := clickSolicitarCita(page, t); err != nil {
			return nil, fmt.Errorf("click Solicitar Cita: %w", err)
		}

		// Wait for next page after clicking "Solicitar Cita"
		time.Sleep(time.Duration(5000+rand.IntN(3000)) * time.Millisecond)

		if isWAFBlocked(page) {
			return nil, fmt.Errorf("WAF blocked after Solicitar Cita")
		}

		// Step 7: Handle office selection if it appears
		if hasOffice, _, _ := page.Has(SelOfficeLater); hasOffice {
			log.Printf("[flow] office selection page detected")
			if err := rod.Try(func() {
				// Select first available office option (not the placeholder)
				el := page.Timeout(t).MustElement(SelOfficeLater)
				// Try to select an option — use the first real option
				opts := el.MustElements("option")
				for _, opt := range opts {
					val := opt.MustProperty("value").String()
					if val != "" && val != "0" {
						el.MustSelect(opt.MustText())
						break
					}
				}
			}); err != nil {
				log.Printf("[flow] office selection error (continuing): %v", err)
			}
			longRandomDelay()

			// Click Siguiente if present
			if hasSig, _, _ := page.Has(SelBtnSiguiente); hasSig {
				log.Printf("[flow] clicking Siguiente")
				if err := rod.Try(func() {
					page.Timeout(t).MustElement(SelBtnSiguiente).MustClick()
				}); err != nil {
					return nil, fmt.Errorf("click siguiente: %w", err)
				}
				time.Sleep(time.Duration(5000+rand.IntN(3000)) * time.Millisecond)
			}
		}

		// Re-read the page after proceeding past options/office
		if err := rod.Try(func() {
			bodyText = page.Timeout(t).MustElement("body").MustText()
		}); err != nil {
			return nil, fmt.Errorf("read body after solicitar cita: %w", err)
		}

		if isWAFBlocked(page) {
			return nil, fmt.Errorf("WAF blocked at final availability check")
		}

		// Now check for the actual availability result
		if containsNoAvailability(bodyText) {
			log.Printf("[flow] no availability (after Solicitar Cita)")
			result.Available = false
			result.Details = "No availability detected (after Solicitar Cita)"
			takeCheckScreenshot(page, cfg, result)
			return result, nil
		}
	}

	// If we're here, page has content without "no availability" text
	// This means actual appointment slots may be shown
	result.Available = true
	result.Details = fmt.Sprintf("Slots may be available!\nPage text snippet: %s", truncate(bodyText, 500))
	takeCheckScreenshot(page, cfg, result)
	return result, nil
}

// isOptionsPage checks if the current page is the intermediate options page
// with "Solicitar Cita", "Consultar Citas Confirmadas", etc.
func isOptionsPage(page *rod.Page, bodyText string) bool {
	// Check by page text content
	if strings.Contains(bodyText, SelOptionsPageText) {
		return true
	}
	// Check by "Solicitar Cita" button presence
	if has, _, _ := page.Has(SelBtnSolicitarCita); has {
		return true
	}
	// Also try finding by button text (in case the ID is different)
	has, _, _ := page.Has("input[value='Solicitar Cita']")
	return has
}

// clickSolicitarCita clicks the "Solicitar Cita" button on the options page.
// Tries multiple selectors since the exact button ID may vary.
func clickSolicitarCita(page *rod.Page, t time.Duration) error {
	// Try by ID first
	if has, _, _ := page.Has(SelBtnSolicitarCita); has {
		return rod.Try(func() {
			page.Timeout(t).MustElement(SelBtnSolicitarCita).MustClick()
		})
	}
	// Try input[value='Solicitar Cita']
	if has, _, _ := page.Has("input[value='Solicitar Cita']"); has {
		return rod.Try(func() {
			page.Timeout(t).MustElement("input[value='Solicitar Cita']").MustClick()
		})
	}
	// Try button/a with text
	if has, _, _ := page.Has("a[value='Solicitar Cita']"); has {
		return rod.Try(func() {
			page.Timeout(t).MustElement("a[value='Solicitar Cita']").MustClick()
		})
	}
	// Last resort: use ElementR for regex text match on any clickable element
	return rod.Try(func() {
		page.Timeout(t).MustElementR("a, button, input[type='submit'], input[type='button']", "Solicitar Cita").MustClick()
	})
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
