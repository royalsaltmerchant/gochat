# Landing Page Redesign — Cipher Editorial

**Date**: 2026-02-15
**Scope**: `index.html` (landing) + `chat_how_it_works.html` (how-it-works)
**Audience**: Privacy-conscious users who want encryption specifics presented accessibly
**Aesthetic**: Elevated dark theme using existing parch-core.css tokens, editorial typography, scroll-driven animations

## Visual Direction

- **Typography**: Playfair Display (headings) + Source Serif 4 (body) from Google Fonts, replacing Georgia
- **Colors**: Existing parch-core.css palette — no new colors
- **Animations**: Intersection Observer fade+slide-up per section, staggered card grids, step-by-step encryption flow reveal
- **Layout**: Full-viewport hero, asymmetric section layouts, generous negative space

## Landing Page (index.html)

### Hero (full viewport)
- Bold headline: "Your messages. Your keys. Your server."
- One-sentence subtitle about decentralized E2EE chat + on-demand video calls
- CTAs: "Open Web Chat" (primary), "Start a Call" (secondary)
- "How It Works" text link below
- Enhanced radial gradient background with noise texture overlay

### Services Section
- Two product cards side by side: Chat + Calls
- **Chat**: E2EE, public-key identity, browser-generated keys, invite-by-public-key, tamper detection
- **Calls**: On-demand video rooms, no scheduling, free 40-min tier, WebRTC
- Each card has action link

### Encryption Showcase
- Horizontal flow diagram (CSS/SVG, not image): plaintext -> AES-GCM -> ECDH+HKDF key wrapping -> Ed25519 signature -> sealed envelope
- Each step is a card/node with algorithm badge + one-line explanation
- Three highlights below: "Zero-knowledge relay", "Forward secrecy per message", "Tamper-proof signatures"

### Trust & Architecture
- Three columns: Browser (plaintext) -> Relay (ciphertext routing) -> Host (ciphertext storage)
- Icon per layer, clear "plaintext boundary" visualization

### Footer
- Links to How It Works, Call Pricing, copyright

## How It Works Page (chat_how_it_works.html)

### Hero
- "How Parch Chat Works"
- Subtitle for privacy-conscious users
- Two-sentence summary

### Section 1: Identity
- Visual of key generation: browser -> Ed25519 + ECDH P-256
- Public key = shareable ID, private key = device-only
- Export/import for device transfer
- Badge: "No central password database"

### Section 2: Authentication
- Challenge-response flow visual
- Steps: Host sends challenge -> Browser signs with Ed25519 -> Host verifies
- Explains binding encryption key to auth challenge
- Badge: "Zero-knowledge authentication"

### Section 3: Encryption
- Detailed envelope breakdown with 4 steps:
  1. Generate unique AES-256-GCM key
  2. Encrypt content -> ciphertext + IV
  3. Wrap AES key per recipient via ECDH + HKDF
  4. Sign envelope with Ed25519
- Styled envelope structure visualization
- Badges: "Per-message forward secrecy", "Authenticated encryption"

### Section 4: Trust Boundaries
- Three-column comparison: Browser / Relay / Host
- What each layer can and cannot see
- Clear "plaintext boundary" stops at browser

### Section 5: Important Notes
- Back up identity file
- New members can't read old messages
- Tampered messages rejected automatically
- Styled callout block

## Tech Decisions

- Plain HTML + CSS + vanilla JS (no build step, consistent with current approach)
- Google Fonts loaded via `<link>` tags
- Intersection Observer API for scroll animations (no library)
- parch-core.css tokens for all colors
- Existing page-level `<style>` pattern (no new CSS files)
- SEO meta tags + Open Graph preserved and updated

## Files Modified

- `relay_server/static/index.html` — full rewrite
- `relay_server/static/chat_how_it_works.html` — full rewrite
