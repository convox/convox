package k8s

// ResetPromCircuitBreakerForTest clears the package-level
// promCircuitBreaker state. Tests that exercise the circuit-breaker
// path must call this in t.Cleanup to leave a fresh breaker for the
// next test in the suite — without it, the threshold counter persists
// across tests and produces nondeterministic results. Test-only.
func ResetPromCircuitBreakerForTest() {
	promCircuitBreaker.Reset()
}
