package main

import (
	"context"
	"fmt"
	"golang.org/x/term"
	"io"
	"net/http"
	"os"

	"github.com/rs/zerolog/log"

	"nhooyr.io/websocket"
)

type rw struct {
	r   io.Reader
	tty *os.File
}

func (r rw) Read(p []byte) (n int, err error) {
	log.Printf("READ START")
	n, err = r.tty.Read(p)
	log.Printf("READ END!!!: %s", p)
	return n, err
}
func (r rw) Write(p []byte) (n int, err error) {
	log.Printf("WRITE START")
	n, err = r.tty.Write(p)
	log.Printf("WRITE!!!: %s", p)
	return n, err
}

type proxy struct{}

func (s *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	log.Debug().Msgf("METHOD: %s", r.Method)
	if r.Method == "OPTIONS" {
		return
	}

	ptyName := r.RequestURI
	tty, err := os.OpenFile("/dev"+ptyName, os.O_RDWR, os.ModePerm)
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

	rwriter := rw{tty: tty}
	terminal := term.NewTerminal(rwriter, ">")

	log.Debug().Msgf("Websocket connection init to: %d", fd)

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		//Subprotocols:       []string{"echo"},
		InsecureSkipVerify: true,
		OriginPatterns:     []string{"*"},
	})
	if err != nil {
		log.Err(err).Msgf("failed to accept connection")
		return
	}
	defer c.CloseNow()

	//if c.Subprotocol() != "echo" {
	//	c.Close(websocket.StatusPolicyViolation, "client must speak the echo subprotocol")
	//	log.Err(err).Msgf("client must speak the echo subprotocol")
	//	return
	//}

	for {
		log.Printf("new iteration")
		err = handlePty(r.Context(), c, terminal)
		if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
			return
		}

		if err != nil {
			log.Printf("failed to echo with %v: %v", r.RemoteAddr, err)
			return
		}
	}
}

func handlePty(ctx context.Context, c *websocket.Conn, terminal *term.Terminal) error {
	typ, r, err := c.Reader(ctx)
	if err != nil {
		return fmt.Errorf("failed to get reader")
	}

	w, err := c.Writer(ctx, typ)
	if err != nil {
		return fmt.Errorf("faield to get writer")
	}

	go func() {
		if _, err = io.Copy(terminal, r); err != nil {
			log.Err(err).Msgf("failed to copy to terminal")
		}
	}()

	log.Printf("tgermianl! %+v", terminal)
	line, err := terminal.ReadLine()
	if err != nil {
		log.Error().Msgf("failed to read line: %v", err)
	}
	println("line:", line)

	//bufio.NewReadWriter(buf, buf)
	//
	//go func() {
	//	var i int64
	//
	//	io.Copy(terminal, reader)
	//	if i, err = io.Copy(w, tty); err != nil {
	//		log.Printf("from tty %v", i)
	//		log.Err(err).Msgf("failed to copy from pty to websocket writer")
	//	}
	//}()
	//var i int64
	//time.Sleep(time.Second / 2)
	//if _, err = io.Copy(tty, r); err != nil {
	//	log.Printf("to tty %v", i)
	//	log.Err(err).Msgf("failed to copy from websocket reader to tty")
	//}
	if err = w.Close(); err != nil {
		return fmt.Errorf("failed to close tty writer: %w", err)
	}
	return nil
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
