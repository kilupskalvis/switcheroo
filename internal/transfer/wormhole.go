// Package transfer moves a bundle between two machines over magic-wormhole.
package transfer

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/psanford/wormhole-william/rendezvous"
	"github.com/psanford/wormhole-william/wormhole"
)

// AppID keeps switcheroo codes apart from other magic-wormhole clients on the same relay.
const AppID = "github.com/kilupskalvis/switcheroo"

const bundleName = "switcheroo.tgz"

// ErrCodeGone means no sender is waiting on the code.
var ErrCodeGone = errors.New("no sender is waiting on this code: it was already used, mistyped, or has expired")

// handshakeTimeout bounds the wait for a sender; a live one answers within seconds.
var handshakeTimeout = 30 * time.Second

// Config selects the relays; empty fields use the public magic-wormhole servers.
type Config struct {
	RendezvousURL string
	TransitRelay  string
	// ListCodes says the relay answers code listings, so dead codes fail at once; always on for the public relay.
	ListCodes bool
}

// Wormhole offers and receives bundles under single-use codes.
type Wormhole struct {
	client    wormhole.Client
	listCodes bool
}

// New returns a Wormhole using the given relays.
func New(cfg Config) *Wormhole {
	return &Wormhole{
		client: wormhole.Client{
			AppID:               AppID,
			RendezvousURL:       cfg.RendezvousURL,
			TransitRelayAddress: cfg.TransitRelay,
		},
		listCodes: cfg.ListCodes || cfg.RendezvousURL == "",
	}
}

// Offer makes r available under a new code; done yields nil once the receiver has all of it.
func (w *Wormhole) Offer(ctx context.Context, r io.ReadSeeker) (code string, done <-chan error, err error) {
	code, status, err := w.client.SendFile(ctx, bundleName, r)
	if err != nil {
		return "", nil, err
	}
	result := make(chan error, 1)
	go func() {
		res, ok := <-status
		switch {
		case !ok:
			result <- errors.New("transfer ended without a result")
		case res.OK:
			result <- nil
		case res.Error != nil:
			result <- res.Error
		default:
			result <- errors.New("transfer failed")
		}
	}()
	return code, result, nil
}

// Receive opens the offer behind code; anything but a switcheroo bundle is refused.
func (w *Wormhole) Receive(ctx context.Context, code string) (io.Reader, error) {
	if w.listCodes {
		active, err := w.codeActive(ctx, code)
		if err != nil {
			return nil, err
		}
		if !active {
			return nil, ErrCodeGone
		}
	}
	// A dead code waits forever for a sender, so only the handshake is timed; the read keeps ctx.
	ctx, cancel := context.WithCancel(ctx)
	timer := time.AfterFunc(handshakeTimeout, cancel)
	msg, err := w.client.Receive(ctx, code)
	if !timer.Stop() {
		cancel()
		return nil, ErrCodeGone
	}
	if err != nil {
		cancel()
		return nil, err
	}
	if msg.Type != wormhole.TransferFile || msg.Name != bundleName {
		_ = msg.Reject()
		cancel()
		return nil, fmt.Errorf("code %s does not carry a switcheroo bundle", code)
	}
	return &cancelOnEnd{Reader: msg, cancel: cancel}, nil
}

// codeActive reports whether the relay still lists the code's nameplate; a picked-up code is released.
func (w *Wormhole) codeActive(ctx context.Context, code string) (bool, error) {
	nameplate, _, ok := strings.Cut(code, "-")
	if !ok {
		return false, nil
	}
	url := w.client.RendezvousURL
	if url == "" {
		url = wormhole.DefaultRendezvousURL
	}
	ctx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()

	rc := rendezvous.NewClient(url, rand.Text(), AppID)
	if _, err := rc.Connect(ctx); err != nil {
		return false, fmt.Errorf("connect to relay: %w", err)
	}
	defer func() { _ = rc.Close(ctx, rendezvous.Happy) }()
	active, err := rc.ListNameplates(ctx)
	if err != nil {
		return false, fmt.Errorf("list codes on relay: %w", err)
	}
	return slices.Contains(active, nameplate), nil
}

// cancelOnEnd releases the receive context once the bundle has been read.
type cancelOnEnd struct {
	io.Reader
	cancel context.CancelFunc
}

func (r *cancelOnEnd) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err != nil {
		r.cancel()
	}
	return n, err
}
