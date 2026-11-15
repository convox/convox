package rack

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	generatedValue   = "a1B2c3D4e5F6g7H8i9J0k1L2m3N4o5P6q7R8s9T0u1V2w3X4y5Z6a7B8c9D0e1F2"
	randomStringLine = "module.system.module.rack.module.api.module.k8s.random_string.password: "
)

func unexpectedTerraform(_, _ io.Writer, args ...string) error {
	return fmt.Errorf("unexpected terraform %s", strings.Join(args, " "))
}

func TestMain(m *testing.M) {
	TestTerraform = unexpectedTerraform
	os.Exit(m.Run())
}

func stubTerraform(t *testing.T, fn func(stdout, stderr io.Writer, args ...string) error) {
	t.Helper()

	TestTerraform = fn
	t.Cleanup(func() { TestTerraform = unexpectedTerraform })
}

func redactLines(r *redactor, lines ...string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = string(r.line([]byte(line)))
	}
	return out
}

func TestRedactorRandomStringLines(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "create refresh destroy",
			in: []string{
				randomStringLine + "Creation complete after 0s [id=" + generatedValue + "]",
				randomStringLine + "Refreshing state... [id=" + generatedValue + "]",
				randomStringLine + "Destroying... [id=" + generatedValue + "]",
				randomStringLine + "Destruction complete after 0s",
			},
			want: []string{
				randomStringLine + "Creation complete after 0s [id=<redacted>]",
				randomStringLine + "Refreshing state... [id=<redacted>]",
				randomStringLine + "Destroying... [id=<redacted>]",
				randomStringLine + "Destruction complete after 0s",
			},
		},
		{
			name: "elapsed suffix is kept",
			in: []string{
				randomStringLine + "Still destroying... [id=" + generatedValue + ", 10s elapsed]",
				randomStringLine + "Still destroying... [id=" + generatedValue + ", 1m10s elapsed]",
			},
			want: []string{
				randomStringLine + "Still destroying... [id=<redacted>, 10s elapsed]",
				randomStringLine + "Still destroying... [id=<redacted>, 1m10s elapsed]",
			},
		},
		{
			name: "brackets inside the value",
			in: []string{
				"module.system.module.cluster.random_string.password: Refreshing state... [id=XXIPd[F]s[9uwjC1Xc]0Fna]03ZR[5]",
				"module.system.module.cluster.random_string.password: Destroying... [id=]MXq]QNNC19TehmQr[1[1[cz]APPgj]",
				"module.system.module.cluster.random_string.password: Destroying... [id=[]T]pA3im[]vIIHG4Y]zNFNXnhktpm]",
			},
			want: []string{
				"module.system.module.cluster.random_string.password: Refreshing state... [id=<redacted>]",
				"module.system.module.cluster.random_string.password: Destroying... [id=<redacted>]",
				"module.system.module.cluster.random_string.password: Destroying... [id=<redacted>]",
			},
		},
		{
			name: "trailing whitespace",
			in: []string{
				randomStringLine + "Refreshing state... [id=" + generatedValue + "] ",
				randomStringLine + "Refreshing state... [id=" + generatedValue + "]\r",
			},
			want: []string{
				randomStringLine + "Refreshing state... [id=<redacted>] ",
				randomStringLine + "Refreshing state... [id=<redacted>]\r",
			},
		},
		{
			name: "unterminated id is cut",
			in:   []string{randomStringLine + "Refreshing state... [id=" + generatedValue},
			want: []string{randomStringLine + "Refreshing state... [id=<redacted>]"},
		},
		{
			name: "naming suffix is redacted on its line and readable elsewhere",
			in: []string{
				"module.system.module.rack.module.api.random_string.suffix: Refreshing state... [id=abcdefghijkl]",
				`      + name = "myrack-storage-abcdefghijkl"`,
			},
			want: []string{
				"module.system.module.rack.module.api.random_string.suffix: Refreshing state... [id=<redacted>]",
				`      + name = "myrack-storage-abcdefghijkl"`,
			},
		},
		{
			name: "other resources keep their id",
			in: []string{
				"module.system.module.cluster.helm_release.karpenter[0]: Modifications complete after 1m7s [id=karpenter]",
				"module.system.module.cluster.aws_eks_node_group.cluster[0]: Still modifying... [id=myrack:myrack-0, 7m0s elapsed]",
			},
			want: []string{
				"module.system.module.cluster.helm_release.karpenter[0]: Modifications complete after 1m7s [id=karpenter]",
				"module.system.module.cluster.aws_eks_node_group.cluster[0]: Still modifying... [id=myrack:myrack-0, 7m0s elapsed]",
			},
		},
		{
			name: "learned value is masked on later lines",
			in: []string{
				randomStringLine + "Refreshing state... [id=" + generatedValue + "]",
				`      - id          = "` + generatedValue + `" -> null`,
				`      - result      = "` + generatedValue + `" -> null`,
				`      ~ id          = "` + generatedValue + `" -> (known after apply)`,
				`          ~ value = "` + generatedValue + `" -> "http://metrics-scraper.kube-system.svc.cluster.local:8000"`,
				`          - value = "` + generatedValue + `" -> null`,
			},
			want: []string{
				randomStringLine + "Refreshing state... [id=<redacted>]",
				`      - id          = "<redacted>" -> null`,
				`      - result      = "<redacted>" -> null`,
				`      ~ id          = "<redacted>" -> (known after apply)`,
				`          ~ value = "<redacted>" -> "http://metrics-scraper.kube-system.svc.cluster.local:8000"`,
				`          - value = "<redacted>" -> null`,
			},
		},
		{
			name: "value is not masked before it is learned",
			in:   []string{`          - value = "` + generatedValue + `" -> null`},
			want: []string{`          - value = "` + generatedValue + `" -> null`},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, redactLines(&redactor{}, tc.in...))
		})
	}
}

func TestRedactorSeed(t *testing.T) {
	r := &redactor{}
	r.seed("dockerhub-pass", "short", "", `pa"ss\word1234`, "pa${ss}word")

	assert.Equal(t, []string{
		`          - value = "<redacted>" -> null`,
		`      ~ "docker_hub_password" = "<redacted>" -> ""`,
		`          - value = "short" -> null`,
		`  12:   docker_hub_password = "<redacted>"`,
		`      ~ "docker_hub_password" = "<redacted>" -> ""`,
		`  12:   docker_hub_password = "<redacted>"`,
	}, redactLines(r,
		`          - value = "dockerhub-pass" -> null`,
		`      ~ "docker_hub_password" = "pa\"ss\\word1234" -> ""`,
		`          - value = "short" -> null`,
		`  12:   docker_hub_password = "pa\"ss\\word1234"`,
		`      ~ "docker_hub_password" = "pa${ss}word" -> ""`,
		`  12:   docker_hub_password = "pa$${ss}word"`,
	))
}

func TestTerraformSecrets(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "vars.json"), []byte(`{"docker_hub_password":"dockerhub-pass","private_eks_pass":"proxy-pass-1","region":"us-east-1"}`), 0600))
	t.Chdir(dir)

	t.Setenv("DIGITALOCEAN_SECRET_KEY", "spaces-secret-key")
	t.Setenv("DIGITALOCEAN_TOKEN", "")

	assert.ElementsMatch(t, []string{"dockerhub-pass", "proxy-pass-1", "spaces-secret-key", ""}, terraformSecrets())
}

func TestTerraformSecretsWithoutVarsFile(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("DIGITALOCEAN_SECRET_KEY", "")
	t.Setenv("DIGITALOCEAN_TOKEN", "")

	assert.ElementsMatch(t, []string{"", "", "", ""}, terraformSecrets())
}

func TestRedactWriterHoldsPartialLines(t *testing.T) {
	var out bytes.Buffer
	w := &redactWriter{red: &redactor{}, w: &out}

	line := randomStringLine + "Refreshing state... [id=" + generatedValue + "]\n"

	n, err := w.Write([]byte(line[:90]))
	require.NoError(t, err)
	assert.Equal(t, 90, n)
	assert.Empty(t, out.String())

	n, err = w.Write([]byte(line[90:] + "Plan: 0 to add"))
	require.NoError(t, err)
	assert.Equal(t, len(line)-90+14, n)
	assert.Equal(t, randomStringLine+"Refreshing state... [id=<redacted>]\n", out.String())

	w.Flush()
	assert.Equal(t, randomStringLine+"Refreshing state... [id=<redacted>]\nPlan: 0 to add", out.String())

	w.Flush()
	assert.Equal(t, randomStringLine+"Refreshing state... [id=<redacted>]\nPlan: 0 to add", out.String())
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestRedactWriterIgnoresDownstreamErrors(t *testing.T) {
	w := &redactWriter{red: &redactor{}, w: failingWriter{}}

	n, err := w.Write([]byte("one\ntwo"))
	require.NoError(t, err)
	assert.Equal(t, 7, n)

	w.Flush()
}

type panicOnceWriter struct {
	mu       sync.Mutex
	panicked bool
	buf      bytes.Buffer
}

func (w *panicOnceWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.panicked {
		w.panicked = true
		panic("boom")
	}

	return w.buf.Write(p)
}

func (w *panicOnceWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.buf.String()
}

func TestRedactWriterRecoversFromPanic(t *testing.T) {
	red := &redactor{}
	broken := &panicOnceWriter{}
	w := &redactWriter{red: red, w: broken}

	n, err := w.Write([]byte("one\n"))
	require.NoError(t, err)
	assert.Equal(t, 4, n)

	n, err = w.Write([]byte("two\n"))
	require.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, "two\n", broken.String())

	var other bytes.Buffer
	_, err = (&redactWriter{red: red, w: &other}).Write([]byte("three\n"))
	require.NoError(t, err)
	assert.Equal(t, "three\n", other.String())
}

func TestRedactWriterKeepsPartialLineAcrossPanic(t *testing.T) {
	broken := &panicOnceWriter{}
	w := &redactWriter{red: &redactor{}, w: broken}

	_, err := w.Write([]byte("one\n" + randomStringLine + "Refreshing state... [id=" + generatedValue[:20]))
	require.NoError(t, err)

	_, err = w.Write([]byte(generatedValue[20:] + "]\n      - id = \"" + generatedValue + "\" -> null\n"))
	require.NoError(t, err)

	assert.Equal(t, randomStringLine+"Refreshing state... [id=<redacted>]\n      - id = \"<redacted>\" -> null\n", broken.String())
}

func TestRedactWriterFlushRecoversFromPanic(t *testing.T) {
	w := &redactWriter{red: &redactor{}, w: &panicOnceWriter{}}

	_, err := w.Write([]byte("no newline"))
	require.NoError(t, err)

	assert.NotPanics(t, w.Flush)
}

func TestRedactWritersShareLearnedValues(t *testing.T) {
	red := &redactor{}

	var stdout, stderr bytes.Buffer
	outw := &redactWriter{red: red, w: &stdout}
	errw := &redactWriter{red: red, w: &stderr}

	_, err := outw.Write([]byte(randomStringLine + "Refreshing state... [id=" + generatedValue + "]\n"))
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_, _ = outw.Write([]byte(randomStringLine + "Creation complete after 0s [id=" + fmt.Sprintf("%02d", i) + generatedValue + "]\n"))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_, _ = errw.Write([]byte("Error: bad value " + generatedValue + "\n"))
		}
	}()

	wg.Wait()

	assert.NotContains(t, stdout.String(), generatedValue)
	assert.NotContains(t, stderr.String(), generatedValue)
	assert.Equal(t, 50, strings.Count(stderr.String(), "Error: bad value <redacted>\n"))
}

func TestCappedBuffer(t *testing.T) {
	b := &cappedBuffer{}

	n, err := b.Write(bytes.Repeat([]byte("a"), stderrCaptureMax))
	require.NoError(t, err)
	assert.Equal(t, stderrCaptureMax, n)
	assert.False(t, b.overflow)

	n, err = b.Write([]byte("b"))
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.True(t, b.overflow)
	assert.Equal(t, stderrCaptureMax, b.buf.Len())
}

func captureStdStreams(t *testing.T, fn func()) (string, string) {
	t.Helper()

	origOut, origErr := os.Stdout, os.Stderr

	outR, outW, err := os.Pipe()
	require.NoError(t, err)
	errR, errW, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout, os.Stderr = outW, errW

	var stdout, stderr []byte
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); stdout, _ = io.ReadAll(outR) }()
	go func() { defer wg.Done(); stderr, _ = io.ReadAll(errR) }()

	fn()

	os.Stdout, os.Stderr = origOut, origErr
	require.NoError(t, outW.Close())
	require.NoError(t, errW.Close())
	wg.Wait()

	return string(stdout), string(stderr)
}

func TestTerraformCaptureRedactsBothStreams(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "vars.json"), []byte(`{"docker_hub_password":"dockerhub-pass"}`), 0600))

	wd, err := os.Getwd()
	require.NoError(t, err)

	var gotArgs []string
	var gotDir string

	stubTerraform(t, func(stdout, stderr io.Writer, args ...string) error {
		gotArgs = args
		gotDir, _ = os.Getwd()

		fmt.Fprintln(stdout, randomStringLine+"Refreshing state... [id="+generatedValue+"]")
		fmt.Fprintln(stdout, `          - value = "dockerhub-pass" -> null`)
		fmt.Fprintln(stderr, "\nError: bad value "+generatedValue)
		fmt.Fprint(stderr, "last line without newline "+generatedValue)

		return errors.New("exit status 1")
	})

	capture := &cappedBuffer{}

	var runErr error
	stdout, stderr := captureStdStreams(t, func() {
		runErr = terraformCapture(dir, capture, "apply", "-auto-approve", "-no-color")
	})

	require.EqualError(t, runErr, "exit status 1")
	assert.Equal(t, []string{"apply", "-auto-approve", "-no-color"}, gotArgs)

	wantDir, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	gotDir, err = filepath.EvalSymlinks(gotDir)
	require.NoError(t, err)
	assert.Equal(t, wantDir, gotDir)

	after, err := os.Getwd()
	require.NoError(t, err)
	assert.Equal(t, wd, after)

	assert.Equal(t, randomStringLine+"Refreshing state... [id=<redacted>]\n          - value = \"<redacted>\" -> null\n", stdout)
	assert.Equal(t, "\nError: bad value <redacted>\nlast line without newline <redacted>", stderr)
	assert.Equal(t, stderr, capture.buf.String())
}
