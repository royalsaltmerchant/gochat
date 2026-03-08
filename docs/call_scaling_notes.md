# Call Roadmap Notes

## Room Access Model

- Default room mode: open.
- Optional room password at creation time.
- If password is enabled:
  - Store only a password hash (Argon2id + salt), never plaintext.
  - Require password on join.
  - Issue short-lived room access token after successful password check.

## Simulcast And Layer Selection

- Simulcast allows each publisher to send multiple video layers (`low`, `medium`, `high`).
- SFU chooses per-subscriber forwarding layer using:
  - receiver network feedback (packet loss, RTT, bitrate estimates),
  - subscriber/client preference (pinned presenter, active speaker, mobile cap),
  - policy constraints (viewport, role, max quality for audience).

## Adaptive Subscribe Policy (App-Level)

- Join on low layer by default for faster/stable startup.
- Promote to high for pinned presenter and active speaker.
- Keep mobile users on low/medium unless explicitly promoted.
- Pause or audio-only subscriptions for off-screen participants.

## Scaling Notes For School-Sized Rooms

- First bottlenecks are typically SFU media node CPU and egress bandwidth.
- Signaling/auth service usually is not the first bottleneck.
- Running multi-node SFU on one machine does not materially increase capacity (same NIC/CPU limits).
- Real scaling requires SFU nodes on separate machines/instances.

## Multi-Node SFU Coordination

- Prefer sticky room placement: one room pinned to one SFU node.
- Use a control plane for:
  - room-to-node mapping,
  - node health/capacity,
  - admission/routing decisions.
- Avoid sharing per-packet RTP state across nodes.
- Rebuild room state by client reconnect on node failure (plus fast failover routing).

## High-Value Features

- Screen share and presenter mode (stage + pin).
- Hand raise and moderated unmute.
- Breakout rooms.
- Audience mode for large classes (few publishers, many subscribers).
- Optional recording bot and live captions.

---

## School Productization Roadmap (Parked Draft)

Status:
- Parked for now.
- Added for future reference if we decide to pursue school/district buyers.
- Last updated: 2026-03-08.

Scope note:
- This is a planning document, not legal advice.
- School procurement typically evaluates product safety controls, compliance posture, and operational reliability more heavily than feature count.

### Target Outcomes By Phase

| Phase | Timeline | Compliance outcomes | Reliability outcomes | Product/sales outcomes |
|---|---|---|---|---|
| Phase 0 | Weeks 0-4 | Data inventory, data classification, draft DPA terms, subprocessor list | Define SLIs/SLOs, incident severity model, on-call + runbooks v1 | Pick one narrow school ICP; avoid broad district motion |
| Phase 1 | Months 2-3 | FERPA-ready controls and contract language | Baseline observability, synthetic call checks | Classroom safety MVP (waiting room, teacher controls) |
| Phase 2 | Months 4-6 | COPPA school-consent workflow, retention/deletion controls, PPRA-safe data use policy | Multi-instance relay/SFU with room affinity, load testing, backup/restore drills | 2-3 paid pilots with weekly reliability reporting |
| Phase 3 | Months 7-12 | WCAG 2.1 AA accessibility posture, pentest, security questionnaire package, SOC 2 Type I prep | Canary deploys, rollback playbooks, error-budget operations | District pilot motion (SSO, admin console, roster sync) |
| Phase 4 | Months 12-18 | SOC 2 Type II observation period, state addenda process | Multi-region disaster recovery, formal SLA | Procurement-ready package for larger districts |

### Compliance Workstream (Detailed)

#### 1) FERPA operating model

- Implement a clear "school official" processor model in policy + contract.
- Ensure school/district retains control of purpose, access, redisclosure, and deletion.
- Build auditable access logs with: org, actor, action, target record, timestamp.
- Support district export and deletion requests with evidence trails.

#### 2) COPPA controls (under-13 contexts)

- Support school-authorized consent in educational use cases.
- Disallow ad-tech or unrelated commercial secondary use of child data.
- Enforce retention minimums and automated deletion against policy windows.

#### 3) PPRA-safe defaults

- Avoid sensitive profiling and marketing use from student data.
- Provide policy toggles/controls for survey or optional data collection features.
- Keep data collection aligned to educational purpose and minimum necessity.

#### 4) Accessibility expectations

- Target WCAG 2.1 AA for core teacher/student workflows.
- Add keyboard-only test passes and screen-reader checks for call-critical screens.
- Track accessibility issues in release criteria, not backlog-only.

#### 5) Trust and procurement artifacts

- DPA template.
- Security overview and architecture/data-flow diagrams.
- Subprocessor list with update process.
- Incident response notification commitments.
- Data retention/deletion policy and deletion certificate workflow.

### Reliability Workstream (Detailed)

#### 1) Service objectives (SLOs)

- Signaling/API availability target (example: 99.95% monthly).
- Join success target (example: >=99.5%).
- p95 join-to-first-remote-media target (example: <8s).
- Unexpected call drop target (example: <1%).
- Sev-1 MTTD/MTTR targets with on-call ownership.

#### 2) Telemetry and QoE visibility

- Collect per-session WebRTC stats: packet loss, RTT, jitter, bitrate, reconnects.
- Build call health dashboards by org and release version.
- Alert on user-impacting SLO burn, not just infrastructure metrics.

#### 3) Scaling architecture

- Remove single-node assumptions for signaling and media paths.
- Keep sticky room-to-SFU placement (do not distribute per-packet RTP state).
- Add admission control for overloaded media nodes.
- Run N+1 capacity for SFU/TURN at minimum.

#### 4) Safe delivery + operations

- Canary releases with auto rollback on SLO degradation.
- Incident runbooks with escalation matrix and customer communication templates.
- Blameless postmortems with tracked corrective actions.

#### 5) Disaster recovery readiness

- Define and test RTO/RPO objectives.
- Automate backups and run regular restore drills.
- Run game-day exercises for node and region failure scenarios (once multi-region exists).

### 90-Day Starter Backlog (Only If Direction Is Activated)

- Add room roles: teacher/moderator/student.
- Add waiting room + admit/deny flow.
- Add room lock and remove/mute participant controls.
- Add persistent room/session state for horizontal scaling.
- Add metrics endpoint and dashboards for join success/latency/drops.
- Add audit log storage + export.
- Add SSO spike (Google Workspace first).
- Add retention/deletion jobs with org-level policy.
- Add accessibility review pass for top call paths.

### Practical GTM Notes

- Start with one narrow use case (example: tutoring/office-hours) before full district replacement pitch.
- Use paid pilots to drive roadmap ordering.
- Treat reliability proof and compliance process maturity as primary deal enablers.
