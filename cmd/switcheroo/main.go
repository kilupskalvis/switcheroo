// Command switcheroo sends a Claude Code session to another person and picks one up.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/kilupskalvis/switcheroo/internal/claude"
	"github.com/kilupskalvis/switcheroo/internal/pickup"
	"github.com/kilupskalvis/switcheroo/internal/send"
	"github.com/kilupskalvis/switcheroo/internal/session"
	"github.com/kilupskalvis/switcheroo/internal/transfer"
)

const usage = `usage:
  switcheroo send --session ID [--timeout 1h]   send a session; prints a one-time code
                                                 a handoff note can be piped to stdin
  switcheroo pickup [--no-resume] CODE          receive a session and resume it
  switcheroo status                             list handoffs sent from this machine
  switcheroo cancel CODE                        withdraw a code that has not been picked up
send and pickup accept --relay-url URL and --transit-relay HOST:PORT for self-hosted relays
`

var errUsage = errors.New("usage")

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	err := errUsage
	if len(args) > 0 {
		switch args[0] {
		case "send":
			err = runSend(args[1:], stdin, stdout, stderr)
		case "pickup":
			err = runPickup(args[1:], stdout)
		case "serve":
			err = runServe(args[1:], stdout, stderr)
		case "status":
			err = runStatus(args[1:], stdout)
		case "cancel":
			err = runCancel(args[1:], stdout)
		}
	}
	switch {
	case err == nil:
		return 0
	case errors.Is(err, errUsage), errors.Is(err, flag.ErrHelp):
		_, _ = fmt.Fprint(stderr, usage)
		return 2
	default:
		_, _ = fmt.Fprintln(stderr, "switcheroo:", err)
		return 1
	}
}

func runSend(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs, cfg := newFlagSet("send")
	id := fs.String("session", "", "session ID to send")
	timeout := fs.Duration("timeout", time.Hour, "how long the code stays valid")
	if err := fs.Parse(args); err != nil || *id == "" || fs.NArg() > 0 {
		return errUsage
	}
	inClaude := os.Getenv("CLAUDECODE") != ""
	note, err := readNote(stdin)
	if err != nil {
		return fmt.Errorf("read handoff note: %w", err)
	}
	if note == "" && inClaude {
		return errors.New("write a handoff note for the next person and pipe it to stdin, e.g. switcheroo send --session ID <<'EOF' ... EOF")
	}
	root, err := claudeRoot()
	if err != nil {
		return err
	}
	dir, err := cacheDir()
	if err != nil {
		return err
	}

	store := session.NewStore(root)
	if inClaude {
		// Claude Code writes the reply that called us just after the call starts.
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		if err := store.AwaitCall(ctx, *id, "switcheroo send"); err != nil && !errors.Is(err, session.ErrNotFound) {
			_, _ = fmt.Fprintln(stderr, "switcheroo: warning: the latest reply may be missing:", err)
		}
		cancel()
	}
	path, err := send.NewService(store, nil, dir).Snapshot(*id, note)
	if err != nil {
		return err
	}
	logPath := strings.TrimSuffix(path, ".tgz") + ".log"
	code, err := spawnServe(path, serveFlags(*timeout, cfg), logPath)
	if err != nil {
		_ = os.Remove(path)
		return err
	}
	_, err = fmt.Fprintf(stdout, "Pickup code: %s\nOn the receiving machine run: switcheroo pickup %s\nThe code works once and expires in %s. Log: %s\n",
		code, code, *timeout, logPath)
	return err
}

func runServe(args []string, stdout, stderr io.Writer) error {
	fs, cfg := newFlagSet("serve")
	bundle := fs.String("bundle", "", "snapshot to offer")
	timeout := fs.Duration("timeout", time.Hour, "how long the code stays valid")
	if err := fs.Parse(args); err != nil || *bundle == "" {
		return errUsage
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	// switcheroo cancel stops a waiting sender with SIGTERM.
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, os.Interrupt)
	defer stop()

	if err := send.NewService(nil, transfer.New(*cfg), "").Serve(ctx, *bundle, stdout); err != nil {
		return err
	}
	_, err := fmt.Fprintln(stderr, "picked up", time.Now().Format(time.RFC3339))
	return err
}

func runStatus(args []string, stdout io.Writer) error {
	if len(args) > 0 {
		return errUsage
	}
	svc, err := outbox()
	if err != nil {
		return err
	}
	hs, err := svc.Handoffs()
	if err != nil {
		return err
	}
	if len(hs) == 0 {
		_, err = fmt.Fprintln(stdout, "No handoffs sent from this machine.")
		return err
	}
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "CODE\tSTATE\tSENT\tSESSION")
	for _, h := range hs[:min(len(hs), 10)] {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", h.Code, h.State, h.Started.Local().Format("2006-01-02 15:04"), h.Session)
	}
	return w.Flush()
}

func runCancel(args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errUsage
	}
	svc, err := outbox()
	if err != nil {
		return err
	}
	if err := svc.Cancel(args[0]); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "Cancelled %s; the code no longer works and the snapshot is deleted.\n", args[0])
	return err
}

// outbox returns a send service over this machine's sent handoffs.
func outbox() (*send.Service, error) {
	dir, err := cacheDir()
	if err != nil {
		return nil, err
	}
	return send.NewService(nil, nil, dir), nil
}

func runPickup(args []string, stdout io.Writer) error {
	fs, cfg := newFlagSet("pickup")
	noResume := fs.Bool("no-resume", false, "install only and print the resume command")
	if err := fs.Parse(args); err != nil || fs.NArg() != 1 {
		return errUsage
	}
	root, err := claudeRoot()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	svc := pickup.NewService(transfer.New(*cfg), session.NewStore(root), claude.NewCLI("claude"), stdout)
	return svc.Pickup(ctx, fs.Arg(0), cwd, !*noResume)
}

// newFlagSet returns a silent flag set with the relay flags every command shares.
func newFlagSet(name string) (*flag.FlagSet, *transfer.Config) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	cfg := &transfer.Config{}
	fs.StringVar(&cfg.RendezvousURL, "relay-url", "", "rendezvous relay URL")
	fs.StringVar(&cfg.TransitRelay, "transit-relay", "", "transit relay host:port")
	return fs, cfg
}

const maxNote = 64 << 10

// readNote reads the handoff note piped to stdin; a terminal or /dev/null means no note.
func readNote(stdin io.Reader) (string, error) {
	if f, ok := stdin.(*os.File); ok {
		if info, err := f.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			return "", nil
		}
	}
	b, err := io.ReadAll(io.LimitReader(stdin, maxNote))
	return strings.TrimSpace(string(b)), err
}

func serveFlags(timeout time.Duration, cfg *transfer.Config) []string {
	flags := []string{"--timeout", timeout.String()}
	if cfg.RendezvousURL != "" {
		flags = append(flags, "--relay-url", cfg.RendezvousURL)
	}
	if cfg.TransitRelay != "" {
		flags = append(flags, "--transit-relay", cfg.TransitRelay)
	}
	return flags
}

// claudeRoot is Claude Code's config dir: $CLAUDE_CONFIG_DIR or ~/.claude.
func claudeRoot() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

func cacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "switcheroo")
	return dir, os.MkdirAll(dir, 0o700)
}
