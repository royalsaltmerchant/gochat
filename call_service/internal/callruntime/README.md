# internal/callruntime

Owns call-session runtime authorization and orchestration logic:
- Room/session join authorization
- Join token validation
- Session access windows and role checks
- Call runtime policy checks for paid sessions

Transport-specific websocket code can stay in adapters, but business policy belongs here.
