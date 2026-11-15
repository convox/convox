package rack

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	redactedText     = "<redacted>"
	learnedSecretMin = 16
	seededSecretMin  = 8
	stderrCaptureMax = 1 << 20
)

var (
	errTerraformInterrupted = errors.New("terraform interrupted")

	terraformDrainIdle = 15 * time.Second

	randomStringMarker = []byte("random_string.")
	randomStringIDOpen = []byte("[id=")
	randomStringID     = regexp.MustCompile(`\[id=(.*?)(?:, [0-9hms]+ elapsed)?\]\s*$`)
)

// For a random_string the id terraform prints is the generated value.
type redactor struct {
	mu      sync.Mutex
	secrets [][]byte
}

func (r *redactor) add(value []byte, floor int) {
	if len(value) < floor {
		return
	}

	for _, s := range r.secrets {
		if bytes.Equal(s, value) {
			return
		}
	}

	r.secrets = append(r.secrets, append([]byte(nil), value...))
}

func (r *redactor) seed(values ...string) {
	for _, v := range values {
		if len(v) < seededSecretMin {
			continue
		}

		// terraform prints plan values and main.tf source lines escaped
		q := strconv.Quote(v)

		for _, form := range []string{v, q[1 : len(q)-1], hclEscape(v)} {
			r.add([]byte(form), seededSecretMin)
		}
	}
}

func (r *redactor) line(line []byte) []byte {
	r.mu.Lock()
	defer r.mu.Unlock()

	if bytes.Contains(line, randomStringMarker) {
		if m := randomStringID.FindSubmatchIndex(line); m != nil {
			r.add(line[m[2]:m[3]], learnedSecretMin)
			line = bytes.Join([][]byte{line[:m[2]], line[m[3]:]}, []byte(redactedText))
		} else if i := bytes.Index(line, randomStringIDOpen); i >= 0 {
			line = append(append([]byte(nil), line[:i]...), "[id="+redactedText+"]"...)
		}
	}

	for _, s := range r.secrets {
		line = bytes.ReplaceAll(line, s, []byte(redactedText))
	}

	return line
}

// redactWriter never reports a failed or short write: os/exec would close the
// pipe and terraform would die on its next write.
type redactWriter struct {
	red  *redactor
	w    io.Writer
	held []byte
}

func (rw *redactWriter) Write(p []byte) (n int, err error) {
	defer func() {
		if recover() != nil {
			rw.held = append([]byte(nil), rw.held[bytes.LastIndexByte(rw.held, '\n')+1:]...)
		}
		n, err = len(p), nil
	}()

	rw.held = append(rw.held, p...)

	end := bytes.LastIndexByte(rw.held, '\n') + 1
	if end == 0 {
		return
	}

	var out []byte
	for _, line := range bytes.Split(rw.held[:end-1], []byte("\n")) {
		out = append(append(out, rw.red.line(line)...), '\n')
	}

	rw.held = append([]byte(nil), rw.held[end:]...)

	_, _ = rw.w.Write(out)

	return
}

func (rw *redactWriter) Flush() {
	defer func() {
		_ = recover()
		rw.held = nil
	}()

	if len(rw.held) > 0 {
		_, _ = rw.w.Write(rw.red.line(rw.held))
	}
}

type cappedBuffer struct {
	buf      bytes.Buffer
	overflow bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.buf.Len()+len(p) > stderrCaptureMax {
		b.overflow = true
	} else {
		b.buf.Write(p)
	}

	return len(p), nil
}

func terraformSecrets() []string {
	vars := map[string]string{}

	if data, err := os.ReadFile("vars.json"); err == nil {
		_ = json.Unmarshal(data, &vars)
	}

	return []string{
		vars["docker_hub_password"],
		vars["private_eks_pass"],
		os.Getenv("DIGITALOCEAN_SECRET_KEY"),
		os.Getenv("DIGITALOCEAN_TOKEN"),
	}
}

func runTerraform(stdout, stderr io.Writer, args ...string) error {
	outr, outw, err := os.Pipe()
	if err != nil {
		return err
	}
	defer outr.Close()

	errr, errw, err := os.Pipe()
	if err != nil {
		outw.Close()
		return err
	}
	defer errr.Close()

	cmd := exec.Command("terraform", args...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = outw
	cmd.Stderr = errw

	term := make(chan os.Signal, 1)
	signal.Notify(term, syscall.SIGTERM)
	defer signal.Stop(term)

	err = cmd.Start()

	outw.Close()
	errw.Close()

	if err != nil {
		return err
	}

	var exited atomic.Bool
	var wg sync.WaitGroup

	wg.Go(func() { drainTerraform(outr, stdout, &exited) })
	wg.Go(func() { drainTerraform(errr, stderr, &exited) })

	err = cmd.Wait()

	exited.Store(true)
	_ = outr.SetReadDeadline(time.Now().Add(terraformDrainIdle))
	_ = errr.SetReadDeadline(time.Now().Add(terraformDrainIdle))

	wg.Wait()

	// terraform 0.13 exits 0 after a graceful stop
	select {
	case <-term:
		return errTerraformInterrupted
	default:
	}

	return err
}

// A process that inherited the pipe can outlive terraform, so once terraform
// has exited a read may only sit idle for terraformDrainIdle.
func drainTerraform(r *os.File, w io.Writer, exited *atomic.Bool) {
	buf := make([]byte, 32*1024)

	for {
		if exited.Load() {
			_ = r.SetReadDeadline(time.Now().Add(terraformDrainIdle))
		}

		n, err := r.Read(buf)
		if n > 0 {
			_, _ = w.Write(buf[:n])
		}

		if err != nil {
			return
		}
	}
}
