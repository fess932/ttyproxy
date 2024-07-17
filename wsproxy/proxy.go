package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"golang.org/x/term"

	"github.com/rs/zerolog/log"

	"nhooyr.io/websocket"
)

type proxy struct{}

func (s proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ptyName := r.RequestURI
	tty, err := os.Open("/dev" + ptyName)
	if err != nil {
		log.Err(err).Msgf("failed to open tty %s", ptyName)
		return
	}
	fd := int(tty.Fd())
	if !term.IsTerminal(fd) {
		log.Error().Msgf("%v is not a terminal", ptyName)
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		panic(err)
	}
	defer term.Restore(fd, oldState)

	log.Debug().Msgf("Websocket connection init to: %d", fd)

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols: []string{"echo"},
	})
	if err != nil {
		log.Err(err).Msgf("failed to accept connection")
		return
	}
	defer c.CloseNow()

	if c.Subprotocol() != "echo" {
		c.Close(websocket.StatusPolicyViolation, "client must speak the echo subprotocol")
		log.Err(err).Msgf("client must speak the echo subprotocol")
		return
	}

	for {
		err = handlePty(r.Context(), c, tty)
		if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
			return
		}
		if err != nil {
			log.Printf("failed to echo with %v: %v", r.RemoteAddr, err)
			return
		}
	}
}

func handlePty(ctx context.Context, c *websocket.Conn, tty *os.File) error {
	typ, r, err := c.Reader(ctx)
	if err != nil {
		return err
	}

	w, err := c.Writer(ctx, typ)
	if err != nil {
		return err
	}

	go func() {
		io.Copy(w, tty)
	}()
	io.Copy(tty, r)

	return w.Close()
}

func echo(ctx context.Context, c *websocket.Conn) error {
	typ, r, err := c.Reader(ctx)
	if err != nil {
		return err
	}

	w, err := c.Writer(ctx, typ)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, r)
	if err != nil {
		return fmt.Errorf("failed to io.Copy: %w", err)
	}

	err = w.Close()
	return err
}
