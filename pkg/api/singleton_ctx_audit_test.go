package api_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestControllerProvider_AllUseWithContext verifies every s.provider(c) call
// in controllers.go chains .WithContext() for request-scoped context.
func TestControllerProvider_AllUseWithContext(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	controllersPath := filepath.Join(filepath.Dir(thisFile), "controllers.go")

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, controllersPath, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", controllersPath, err)
	}

	var offenders []string
	ast.Inspect(f, func(n ast.Node) bool {
		// Find call expressions: x.f(...)
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		// Look for inner call: s.provider(c)
		inner, ok := sel.X.(*ast.CallExpr)
		if !ok {
			return true
		}
		innerSel, ok := inner.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := innerSel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if ident.Name != "s" || innerSel.Sel.Name != "provider" {
			return true
		}
		// We have a (s.provider(...)).<Method> chain.
		if sel.Sel.Name != "WithContext" {
			pos := fset.Position(sel.Pos())
			offenders = append(offenders, pos.String())
		}
		return true
	})

	assert.Empty(t, offenders,
		"every s.provider(c) call site must chain .WithContext(contextFrom(c)). offenders:\n%s",
		strings.Join(offenders, "\n"))
}
