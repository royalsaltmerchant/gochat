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
