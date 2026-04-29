package common

// SetStreamerExitedHookForTest is the test-side accessor for the streamer-
// exit signal used by TestWaitForAppWithLogsContext_RaceFree and
// TestWaitForRackWithLogs_RaceFree. The hook fires at the end of the streamer
// goroutine inside the wait-with-logs helpers, after the goroutine's `defer
// close(done)` is queued. Tests use it to assert the streamer-exit-before-
// helper-return invariant.
//
// Files ending in _test.go are excluded from production builds, so production
// binaries can never call these accessors and the hook variable always reads
// nil there.
func SetStreamerExitedHookForTest(fn func()) { streamerExitedHookForTest = fn }
func ClearStreamerExitedHookForTest()        { streamerExitedHookForTest = nil }
