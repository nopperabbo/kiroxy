#!/usr/bin/env python3
"""
enowX Warmup Auto-Clicker.

Opens http://localhost:1431/accounts, logs in once, then loops forever
clicking any visible "Warmup" button. Stops when no button found for 
N consecutive cycles OR user hits Ctrl+C.

Usage:
  export ENOWX_LICENSE="ENOWX-DU5U5-YCJ6Q-KOGOY-Z340F"
  export ENOWX_PASSWORD="Gandaria0#21"
  python3 warmup_clicker.py
  
Optional env:
  ENOWX_URL        (default http://localhost:1431)
  WARMUP_INTERVAL  (seconds between scan cycles, default 30)
  HEADLESS         (1 = headless, 0 = visible browser, default 0 so you can see)
  STOP_AFTER_IDLE  (stop after N cycles with no warmup button found, default 999)
"""

import os
import sys
import time
import signal
from playwright.sync_api import sync_playwright

ENOWX_URL = os.environ.get("ENOWX_URL", "http://localhost:1431")
LICENSE = os.environ.get("ENOWX_LICENSE", "").strip()
PASSWORD = os.environ.get("ENOWX_PASSWORD", "").strip()
INTERVAL = int(os.environ.get("WARMUP_INTERVAL", "30"))
HEADLESS = os.environ.get("HEADLESS", "0") == "1"
STOP_AFTER_IDLE = int(os.environ.get("STOP_AFTER_IDLE", "999"))

if not LICENSE or not PASSWORD:
    print("error: ENOWX_LICENSE and ENOWX_PASSWORD env vars required")
    sys.exit(1)


stop_requested = False
def handle_sig(signum, frame):
    global stop_requested
    stop_requested = True
    print("\n[warmup] stop requested, will exit after current cycle")
signal.signal(signal.SIGINT, handle_sig)
signal.signal(signal.SIGTERM, handle_sig)


def log(msg):
    print(f"[{time.strftime('%H:%M:%S')}] {msg}", flush=True)


def login(page):
    log(f"navigating to {ENOWX_URL}/login")
    page.goto(f"{ENOWX_URL}/login", wait_until="domcontentloaded", timeout=15000)
    
    # Cari field license (bisa ada beberapa pattern)
    license_selectors = [
        'input[name="license"]', 'input[name="license_key"]',
        'input[placeholder*="icense" i]', 'input[placeholder*="key" i]',
        'input[type="text"]:first-of-type'
    ]
    password_selectors = [
        'input[name="password"]', 'input[type="password"]'
    ]
    submit_selectors = [
        'button[type="submit"]', 'button:has-text("Login")',
        'button:has-text("Sign in")', 'button:has-text("Masuk")',
        'button:has-text("Submit")'
    ]

    for sel in license_selectors:
        try:
            if page.locator(sel).count() > 0:
                page.locator(sel).first.fill(LICENSE)
                log(f"filled license via {sel}")
                break
        except Exception:
            continue
    
    for sel in password_selectors:
        try:
            if page.locator(sel).count() > 0:
                page.locator(sel).first.fill(PASSWORD)
                log(f"filled password via {sel}")
                break
        except Exception:
            continue
    
    for sel in submit_selectors:
        try:
            if page.locator(sel).count() > 0:
                page.locator(sel).first.click()
                log(f"clicked submit via {sel}")
                break
        except Exception:
            continue
    
    # Wait for navigation to /accounts or dashboard
    try:
        page.wait_for_url(lambda url: "login" not in url.lower(), timeout=15000)
        log(f"logged in, now at {page.url}")
    except Exception:
        log(f"login nav timeout, current url: {page.url} — proceeding anyway")


def find_warmup_buttons(page):
    """Return all visible+enabled Warmup buttons."""
    # Try multiple selectors — enowX button wording may vary
    selectors = [
        'button:has-text("Warmup")',      # English
        'button:has-text("Warm up")',     # variant
        'button:has-text("Warm Up")',     # variant
        'button:has-text("Panaskan")',    # Indonesian
        '[data-action="warmup"]',         # data attribute
        'button[class*="warmup" i]',      # class-based
    ]
    buttons = []
    for sel in selectors:
        try:
            locator = page.locator(sel)
            count = locator.count()
            for i in range(count):
                btn = locator.nth(i)
                try:
                    if btn.is_visible() and btn.is_enabled():
                        buttons.append(btn)
                except Exception:
                    continue
        except Exception:
            continue
    return buttons


def warmup_cycle(page):
    """One scan + click cycle. Returns count of buttons clicked."""
    try:
        # Make sure we're at /accounts
        if "/accounts" not in page.url:
            log(f"not on /accounts (at {page.url}), navigating")
            page.goto(f"{ENOWX_URL}/accounts", wait_until="domcontentloaded", timeout=10000)
            time.sleep(1)  # let React hydrate
        
        # Refresh to pick up latest state
        page.reload(wait_until="domcontentloaded", timeout=10000)
        time.sleep(1)  # let React hydrate
        
        buttons = find_warmup_buttons(page)
        if not buttons:
            return 0
        
        clicked = 0
        for btn in buttons:
            try:
                # Re-check enabled right before click (state can change)
                if not (btn.is_visible() and btn.is_enabled()):
                    continue
                btn.click(timeout=5000)
                clicked += 1
                time.sleep(0.3)  # small pause between clicks
            except Exception as e:
                log(f"click failed: {e}")
                continue
        return clicked
    except Exception as e:
        log(f"cycle error: {e}")
        return 0


def main():
    log(f"starting warmup clicker")
    log(f"  url={ENOWX_URL}  interval={INTERVAL}s  headless={HEADLESS}  stop_after_idle={STOP_AFTER_IDLE}")
    
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=HEADLESS)
        context = browser.new_context()
        page = context.new_page()
        
        try:
            login(page)
            
            # Navigate to /accounts
            page.goto(f"{ENOWX_URL}/accounts", wait_until="domcontentloaded", timeout=15000)
            time.sleep(2)
            log(f"at accounts page, starting warmup loop")
            
            cycle_num = 0
            total_clicks = 0
            idle_cycles = 0
            
            while not stop_requested:
                cycle_num += 1
                clicks = warmup_cycle(page)
                total_clicks += clicks
                
                if clicks > 0:
                    idle_cycles = 0
                    log(f"cycle {cycle_num}: clicked {clicks} warmup buttons (total: {total_clicks})")
                else:
                    idle_cycles += 1
                    log(f"cycle {cycle_num}: no warmup buttons found (idle streak: {idle_cycles}/{STOP_AFTER_IDLE})")
                
                if idle_cycles >= STOP_AFTER_IDLE:
                    log(f"reached idle threshold, exiting")
                    break
                
                # Sleep in chunks so Ctrl+C is responsive
                for _ in range(INTERVAL):
                    if stop_requested:
                        break
                    time.sleep(1)
            
            log(f"done. total cycles={cycle_num}, total clicks={total_clicks}")
        
        finally:
            browser.close()


if __name__ == "__main__":
    main()
