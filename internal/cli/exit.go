// Stable, documented exit codes for CI and agent integration.
//
//	0 — success
//	1 — runtime error (network, auth, malformed input)
//	2 — lint findings triggered fail-on (msv doctor only)
//	3 — reserved for future mismatch-only exit
//
// The doctor command already uses ExitLintFindings. `main` maps any
// returned error to ExitRuntimeError. Nothing else exits non-zero.
package cli
