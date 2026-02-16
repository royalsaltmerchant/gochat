# Landing Page Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Redesign the main landing page and how-it-works page with an editorial aesthetic that makes Parch's encryption story compelling for privacy-conscious users.

**Architecture:** Two static HTML files (`index.html`, `chat_how_it_works.html`) served by the relay server's existing Gin routes. No build step, no new dependencies. All styling is page-level `<style>` blocks using parch-core.css tokens. Scroll animations via Intersection Observer API.

**Tech Stack:** Plain HTML5 + CSS3 + vanilla JS, Google Fonts (Playfair Display + Source Serif 4), parch-core.css design tokens

---

### Task 1: Rewrite Landing Page (index.html)

**Files:**
- Modify: `relay_server/static/index.html` (full rewrite)

**Step 1: Write the new index.html**

Full rewrite of `relay_server/static/index.html`. The page has these sections:

1. **Hero** (full viewport height): Bold headline "Your messages. Your keys. Your server.", subtitle about decentralized E2EE chat + video calls, two CTA buttons (Open Web Chat → `https://chat.parchchat.com/client`, Start a Call → `/call`), text link to How It Works (`/chat/how-it-works`). Background uses existing radial gradients + parch-noise overlay.

2. **Services** (two product cards): Chat card with E2EE features (public-key identity, browser-generated keys, invite-by-public-key, tamper detection). Calls card with video features (on-demand rooms, free 40-min tier, WebRTC). Each card links to its product.

3. **Encryption Showcase**: Horizontal flow diagram built with CSS flexbox/grid (not images). Five steps: Plaintext → AES-GCM Encrypt → ECDH+HKDF Key Wrap → Ed25519 Sign → Sealed Envelope. Each step is a styled node with algorithm badge + one-line explanation. Connected by arrow/line elements. Three highlight badges below: "Zero-knowledge relay", "Forward secrecy per message", "Tamper-proof signatures".

4. **Trust & Architecture**: Three columns showing trust boundary — Browser (generates keys, encrypts/decrypts, only place plaintext exists), Relay (verifies signatures, routes ciphertext, cannot read messages), Host (stores encrypted envelopes, manages permissions, cannot read messages). Each column has an inline SVG icon.

5. **Footer**: Links to How It Works, Call Pricing, copyright "ParchChat.com 2026".

**Key CSS details:**
- Google Fonts: `Playfair Display:wght@400;700;900` (headings) + `Source Serif 4:wght@400;600` (body)
- All colors from parch-core.css CSS variables (use `--parch-*` names)
- `.reveal` class for scroll-animated elements, driven by Intersection Observer
- Animation: elements start `opacity: 0; transform: translateY(30px)`, transition to visible on intersection
- Staggered delays on card grids using `transition-delay` increments
- Encryption flow nodes animate in sequence with increasing delays
- Hero is `min-height: 100vh` with flex centering
- Responsive: services grid and trust columns stack to single column below 768px, encryption flow wraps vertically on mobile
- Preserve existing SEO meta tags, update og:description and twitter:description to reflect new content

**Step 2: Verify the page renders**

Open `http://localhost:<port>/` in a browser (or serve via `go run` if the relay server is runnable locally). Check:
- Hero fills viewport, CTAs link correctly
- Services cards render side by side on desktop, stack on mobile
- Encryption flow diagram displays horizontally with arrows
- Trust columns render
- Scroll animations fire on each section
- All colors match parch-core.css tokens

**Step 3: Commit**

```bash
git add relay_server/static/index.html
git commit -m "landing: redesign index.html with cipher editorial layout

Replaces developer-focused overview with user-facing landing page.
Hero with E2EE value proposition, services cards, encryption flow
diagram, and trust boundary visualization. Playfair Display +
Source Serif 4 typography, scroll-driven animations."
```

---

### Task 2: Rewrite How It Works Page (chat_how_it_works.html)

**Files:**
- Modify: `relay_server/static/chat_how_it_works.html` (full rewrite)

**Step 1: Write the new chat_how_it_works.html**

Full rewrite of `relay_server/static/chat_how_it_works.html`. The page has these sections:

1. **Back link**: `← Back to Parch` linking to `/`

2. **Hero**: Headline "How Parch Chat Works", subtitle positioning this for privacy-conscious users, two-sentence summary (no email logins, browser-generated identity, only recipients can read messages).

3. **Section 1 — Identity ("Your keys are your identity")**: Visual showing browser generating Ed25519 (signing) + ECDH P-256 (encryption) keypair. Explains public key = shareable ID, private key = device-only. Mentions export/import. Badge: "No central password database".

4. **Section 2 — Authentication ("Prove you own your keys")**: Visual showing challenge-response flow. Three steps: Host sends challenge → Browser signs `parch-chat-auth:<hostUUID>:<challenge>:<encPublicKey>` with Ed25519 → Host verifies signature. Explains binding encryption key to auth challenge prevents key substitution. Badge: "Zero-knowledge authentication".

5. **Section 3 — Encryption ("Messages sealed before they leave")**: Detailed 4-step envelope breakdown. Step 1: Generate unique AES-256-GCM key. Step 2: Encrypt content → ciphertext + IV. Step 3: For each recipient, wrap AES key via ECDH key agreement + HKDF. Step 4: Sign entire envelope with Ed25519. Styled envelope structure visualization showing metadata, ciphertext, wrappedKeys array, signature. Badges: "Per-message forward secrecy", "Authenticated encryption".

6. **Section 4 — Trust Boundaries ("What each layer can see")**: Three-column comparison. Browser: generates keys, encrypts/decrypts, sees plaintext. Relay: verifies signatures, routes envelopes, sees only ciphertext. Host: stores envelopes, manages permissions, sees only ciphertext. Visual distinction showing plaintext boundary stops at browser.

7. **Section 5 — Important Notes**: Callout block with: back up identity file, new members can't read old messages, tampered messages rejected automatically.

8. **Footer**: Copyright, link back to landing.

**Key CSS details:**
- Same Google Fonts as landing page (Playfair Display + Source Serif 4)
- Same parch-core.css tokens
- Same `.reveal` Intersection Observer pattern for scroll animations
- Encryption steps use a vertical or stepped flow layout with connecting lines
- Trust boundary table uses subtle background distinction for the browser column (where plaintext lives)
- Badge elements: small pill-shaped spans with `--parch-green` or `--parch-accent-blue` backgrounds
- Responsive: all multi-column layouts stack on mobile
- Preserve SEO meta tags, update descriptions

**Step 2: Verify the page renders**

Open `http://localhost:<port>/chat/how-it-works` in a browser. Check:
- All 5 sections render with proper hierarchy
- Identity and auth flow visuals display correctly
- Encryption envelope breakdown is readable
- Trust boundary columns show clear distinction
- Scroll animations work
- Back link navigates to landing
- Mobile responsive

**Step 3: Commit**

```bash
git add relay_server/static/chat_how_it_works.html
git commit -m "landing: redesign how-it-works with encryption deep-dive

Replaces basic guide with detailed editorial walkthrough of identity,
authentication, E2EE envelope model, and trust boundaries. Algorithm
specifics (Ed25519, ECDH P-256, AES-GCM, HKDF) presented with
visual flow diagrams and plain-language explanations."
```

---

### Task 3: Final Review and Polish

**Files:**
- Review: `relay_server/static/index.html`
- Review: `relay_server/static/chat_how_it_works.html`

**Step 1: Cross-page consistency check**

Verify both pages share:
- Same Google Fonts imports
- Same animation pattern (`.reveal` + Intersection Observer)
- Same color usage from parch-core.css
- Consistent button/link styles
- Consistent footer
- Navigation between pages works (landing → how-it-works → landing)

**Step 2: Accessibility check**

- All images have alt text
- Color contrast meets WCAG AA against dark backgrounds
- Interactive elements are keyboard-navigable
- Semantic HTML (sections, articles, headings hierarchy)
- `prefers-reduced-motion` media query disables animations

**Step 3: Fix any issues found, commit**

```bash
git add relay_server/static/index.html relay_server/static/chat_how_it_works.html
git commit -m "landing: polish and cross-page consistency fixes"
```
