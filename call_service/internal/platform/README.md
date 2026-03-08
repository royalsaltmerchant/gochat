# internal/platform

Owns cross-cutting platform concerns:
- Service bootstrap helpers
- Shared middleware helpers
- Shared HTTP/JSON response helpers
- Environment/config parsing helpers

Business domains (`auth`, `gm`, `billing`, `callruntime`) should not be implemented here.
