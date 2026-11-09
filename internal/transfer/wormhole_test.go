package transfer

import (
	"bytes"
	"context"
	"crypto/rand"
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
	cfg, ctx := testConfig(t), testContext(t)
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
