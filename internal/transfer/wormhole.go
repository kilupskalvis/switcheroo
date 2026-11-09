// Package transfer moves a bundle between two machines over magic-wormhole.
package transfer

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/psanford/wormhole-william/wormhole"
)

// AppID keeps switcheroo codes apart from other magic-wormhole clients on the same relay.
const AppID = "github.com/kilupskalvis/switcheroo"

const bundleName = "switcheroo.tgz"

// Config selects the relays; empty fields use the public magic-wormhole servers.
type Config struct {
	RendezvousURL string
	TransitRelay  string
}

// Wormhole offers and receives bundles under single-use codes.
type Wormhole struct {
	client wormhole.Client
}

// New returns a Wormhole using the given relays.
func New(cfg Config) *Wormhole {
	return &Wormhole{client: wormhole.Client{
		AppID:               AppID,
		RendezvousURL:       cfg.RendezvousURL,
		TransitRelayAddress: cfg.TransitRelay,
	}}
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
	msg, err := w.client.Receive(ctx, code)
	if err != nil {
		return nil, err
	}
	if msg.Type != wormhole.TransferFile || msg.Name != bundleName {
		_ = msg.Reject()
		return nil, fmt.Errorf("code %s does not carry a switcheroo bundle", code)
	}
	return msg, nil
}
