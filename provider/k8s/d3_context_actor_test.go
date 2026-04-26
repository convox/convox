package k8s_test

import (
	"context"
	"sync"
	"testing"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/assert"
)

// TestContextActor_NilProvider asserts a nil receiver does not panic and
// returns "unknown" so callers can blindly emit audit events.
func TestContextActor_NilProvider(t *testing.T) {
	var p *k8s.Provider
	got := p.ContextActor()
	assert.Equal(t, "unknown", got)
}

// TestContextActor_NilCtx asserts a Provider with no ctx returns "unknown".
// This shape happens when a caller forgets to chain WithContext(...).
func TestContextActor_NilCtx(t *testing.T) {
	p := &k8s.Provider{}
	got := p.ContextActor()
	assert.Equal(t, "unknown", got)
}

// TestContextActor_BackgroundCtx asserts a Provider whose ctx is
// context.Background() (no value populated) returns "unknown".
func TestContextActor_BackgroundCtx(t *testing.T) {
	p := &k8s.Provider{}
	pp, _ := p.WithContext(context.Background()).(*k8s.Provider)
	got := pp.ContextActor()
	assert.Equal(t, "unknown", got)
}

// TestContextActor_PopulatesActorWhenSet asserts a Provider with the param
// stashed in ctx returns the user string verbatim.
func TestContextActor_PopulatesActorWhenSet(t *testing.T) {
	cases := []struct {
		name string
		user string
	}{
		{"system-read", "system-read"},
		{"system-write", "system-write"},
		{"system-admin", "system-admin"},
		// Whitespace propagates verbatim per spec.
		{"whitespace claim", "   "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &k8s.Provider{}
			ctx := context.WithValue(context.Background(), structs.ConvoxJwtUserCtxKey, tc.user)
			pp, _ := p.WithContext(ctx).(*k8s.Provider)
			assert.Equal(t, tc.user, pp.ContextActor())
		})
	}
}

// TestContextActor_EmptyClaim_ReturnsUnknown asserts the empty-string guard
// inside ContextActor maps "" -> "unknown".
func TestContextActor_EmptyClaim_ReturnsUnknown(t *testing.T) {
	p := &k8s.Provider{}
	ctx := context.WithValue(context.Background(), structs.ConvoxJwtUserCtxKey, "")
	pp, _ := p.WithContext(ctx).(*k8s.Provider)
	assert.Equal(t, "unknown", pp.ContextActor())
}

// TestContextActor_NonStringValue_ReturnsUnknown asserts a non-string value
// at the key (defense in depth — middleware always writes string but a
// future bug could write any type) collapses to "unknown".
func TestContextActor_NonStringValue_ReturnsUnknown(t *testing.T) {
	p := &k8s.Provider{}
	ctx := context.WithValue(context.Background(), structs.ConvoxJwtUserCtxKey, 42)
	pp, _ := p.WithContext(ctx).(*k8s.Provider)
	assert.Equal(t, "unknown", pp.ContextActor())
}

// TestContextActor_CancelledCtx asserts ctx.Value lookup survives
// cancellation per Go semantics — the value is still readable.
func TestContextActor_CancelledCtx(t *testing.T) {
	p := &k8s.Provider{}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), structs.ConvoxJwtUserCtxKey, "system-write"))
	cancel()
	pp, _ := p.WithContext(ctx).(*k8s.Provider)
	assert.Equal(t, "system-write", pp.ContextActor(), "ctx.Value must survive cancellation")
}

type unrelatedKeyA struct{}
type unrelatedKeyB struct{}

// TestContextActor_NeverPanics is a property test: arbitrary WithValue chains
// on context.Background() must never panic. We layer many keys, including the
// target one with various shapes.
func TestContextActor_NeverPanics(t *testing.T) {
	p := &k8s.Provider{}
	ctx := context.Background()
	ctx = context.WithValue(ctx, unrelatedKeyA{}, "b")
	ctx = context.WithValue(ctx, structs.ConvoxJwtUserCtxKey, "system-write")
	ctx = context.WithValue(ctx, unrelatedKeyB{}, structs.ConvoxRoleReadWrite)
	pp, _ := p.WithContext(ctx).(*k8s.Provider)
	assert.NotPanics(t, func() { _ = pp.ContextActor() })
}

// TestContextActor_ConcurrentReadsSafe asserts ContextActor is safe under
// concurrent invocation against a single provider instance. Run under -race
// to catch any latent shared-state bugs.
func TestContextActor_ConcurrentReadsSafe(t *testing.T) {
	p := &k8s.Provider{}
	ctx := context.WithValue(context.Background(), structs.ConvoxJwtUserCtxKey, "system-write")
	pp, _ := p.WithContext(ctx).(*k8s.Provider)

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if got := pp.ContextActor(); got != "system-write" {
				t.Errorf("concurrent read got %q want system-write", got)
			}
		}()
	}
	wg.Wait()
}
