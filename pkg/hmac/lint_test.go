package hmac_test

// AST-level guard: this test parses pkg/hmac/hmac.go and verifies that
// Verify uses hmac.Equal (constant-time) and never references bytes.Equal
// for signature comparison. The intent is to catch a future refactor that
// "helpfully" replaces hmac.Equal with bytes.Equal — that swap would
// reintroduce the timing side-channel hmac.Equal exists to prevent.
//
// Spec ref: §8.1 "Constant-time compare (R1 + R2 BLOCK)"; R2 F-T-NEW-3.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLint_VerifyUsesHmacEqual_NotBytesEqual(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "hmac.go", nil, parser.AllErrors)
	require.NoError(t, err, "parse hmac.go")

	var verifyFn *ast.FuncDecl
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Name != nil && fn.Name.Name == "Verify" {
			verifyFn = fn
			break
		}
	}
	require.NotNil(t, verifyFn, "Verify function must exist in hmac.go")

	var (
		usesHmacEqual  bool
		usesBytesEqual bool
	)
	ast.Inspect(verifyFn, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if ident.Name == "hmac" && sel.Sel != nil && sel.Sel.Name == "Equal" {
			usesHmacEqual = true
		}
		if ident.Name == "bytes" && sel.Sel != nil && sel.Sel.Name == "Equal" {
			usesBytesEqual = true
		}
		return true
	})

	assert.True(t, usesHmacEqual, "Verify must call hmac.Equal for constant-time compare")
	assert.False(t, usesBytesEqual, "Verify must NOT call bytes.Equal — timing side-channel")
}

func TestLint_NoLogPackageImports(t *testing.T) {
	// Belt-and-suspenders: pkg/hmac MUST NOT import any logger package.
	// Sign / Verify / SignedHeader are internal primitives; they must
	// NOT emit log lines that could leak key bytes, body bytes, or hex
	// signatures via WARN/INFO/DEBUG. (R1 obs §1.1)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "hmac.go", nil, parser.ImportsOnly)
	require.NoError(t, err)

	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		assert.NotContains(t, path, "logger")
		assert.NotContains(t, path, "logrus")
		assert.NotEqual(t, "log", path)
	}
}
