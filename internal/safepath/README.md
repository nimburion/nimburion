# internal/safepath

Owns internal file-path safety helpers used by framework packages that read
local files.

This package is intentionally internal: it provides plumbing for validated file
access without creating a broad public `pkg/security` surface.
