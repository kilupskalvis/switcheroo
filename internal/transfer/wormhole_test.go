package transfer

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/psanford/wormhole-william/rendezvous/rendezvousservertest"
	"github.com/psanford/wormhole-william/wormhole"
)

func TestMain(m *testing.M) {
	// No transit relay in tests: peers connect directly, as the upstream tests do.
	wormhole.DefaultTransitRelayAddress = ""
	handshakeTimeout = 3 * time.Second
	os.Exit(m.Run())
}

// testConfig points at an in-process rendezvous server.
func testConfig(t *testing.T) Config {
	t.Helper()
	rs := rendezvousservertest.NewServer()
	t.Cleanup(rs.Close)
	return Config{RendezvousURL: rs.WebSocketURL()}
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestOfferReceive(t *testing.T) {
	for _, list := range []bool{false, true} {
		cfg := testConfig(t)
		cfg.ListCodes = list
		offerReceive(t, cfg)
	}
}

func offerReceive(t *testing.T, cfg Config) {
	t.Helper()
	ctx := testContext(t)
	payload := make([]byte, 1<<20)
	_, _ = rand.Read(payload)

	code, done, err := New(cfg).Offer(ctx, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	r, err := New(cfg).Receive(ctx, code)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("received %d bytes, differ from the %d sent", len(got), len(payload))
	}
	if err := <-done; err != nil {
		t.Fatalf("done = %v, want nil", err)
	}
}

func TestReceiveWrongCode(t *testing.T) {
	cfg, ctx := testConfig(t), testContext(t)
	code, done, err := New(cfg).Offer(ctx, bytes.NewReader([]byte("secret")))
	if err != nil {
		t.Fatal(err)
	}
	nameplate := code[:bytes.IndexByte([]byte(code), '-')]

	if _, err := New(cfg).Receive(ctx, nameplate+"-wrong-guess"); err == nil {
		t.Fatal("receive with a wrong code succeeded")
	}
	if err := <-done; err == nil {
		t.Fatal("offer survived a wrong guess")
	}
}

// usedCode returns a code whose bundle has already been picked up.
func usedCode(ctx context.Context, t *testing.T, cfg Config) string {
	t.Helper()
	code, done, err := New(cfg).Offer(ctx, bytes.NewReader([]byte("secret")))
	if err != nil {
		t.Fatal(err)
	}
	r, err := New(cfg).Receive(ctx, code)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(r); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	return code
}

func TestReceiveUsedCode(t *testing.T) {
	cfg, ctx := testConfig(t), testContext(t)
	code := usedCode(ctx, t, cfg)

	if _, err := New(cfg).Receive(ctx, code); !errors.Is(err, ErrCodeGone) {
		t.Fatalf("second receive = %v, want ErrCodeGone", err)
	}
}

func TestReceiveAbandonedCode(t *testing.T) {
	cfg, ctx := testConfig(t), testContext(t)
	senderCtx, stopSender := context.WithCancel(ctx)
	code, _, err := New(cfg).Offer(senderCtx, bytes.NewReader([]byte("secret")))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond) // let the sender post its first message
	stopSender()                       // like a cancelled or killed sender: that message stays in the mailbox

	errs := make(chan error, 1)
	go func() {
		_, err := New(cfg).Receive(ctx, code)
		errs <- err
	}()
	select {
	case err := <-errs:
		if !errors.Is(err, ErrCodeGone) {
			t.Fatalf("receive = %v, want ErrCodeGone", err)
		}
	case <-time.After(handshakeTimeout + 5*time.Second):
		t.Fatal("receive hung past the handshake timeout")
	}
}

func TestReceiveFailsFastWhenRelayListsCodes(t *testing.T) {
	cfg, ctx := testConfig(t), testContext(t)
	cfg.ListCodes = true
	for name, code := range map[string]string{
		"used":    usedCode(ctx, t, cfg),
		"unknown": "7-nobody-here",
	} {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			if _, err := New(cfg).Receive(ctx, code); !errors.Is(err, ErrCodeGone) {
				t.Fatalf("receive = %v, want ErrCodeGone", err)
			}
			if elapsed := time.Since(start); elapsed > handshakeTimeout/3 {
				t.Errorf("took %s, want an answer from the code list", elapsed)
			}
		})
	}
}

func TestReceiveRejectsText(t *testing.T) {
	cfg, ctx := testConfig(t), testContext(t)
	sender := wormhole.Client{AppID: AppID, RendezvousURL: cfg.RendezvousURL, TransitRelayAddress: cfg.TransitRelay}
	code, _, err := sender.SendText(ctx, "not a bundle")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := New(cfg).Receive(ctx, code); err == nil {
		t.Fatal("receive accepted a text offer")
	}
}
