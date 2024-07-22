package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	gotty "github.com/mattn/go-tty"
	"nhooyr.io/websocket"
)

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
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
		OriginPatterns:     []string{"*"},
	})
	if err != nil {
		log.Err(err).Msgf("failed to accept connection")
		return
	}
	defer c.CloseNow()
	read := make(chan []byte)
	write := make(chan []byte)
	go readInput(r.Context(), c, read, write)

	stty, err := gotty.OpenDevice("/dev" + ptyName)
	if err != nil {
		log.Printf("failed to open stty: %v", err)
		return
	}

	backup, err := stty.Raw()
	if err != nil {
		log.Printf("failed to open raw stty: %v", err)
		return
	}
	defer func() {
		if derr := backup(); derr != nil {
			log.Printf("failed to return raw tty backup: %v", derr)
		}
	}()
	{
		x, y, z, v, err := stty.SizePixel()
		if err != nil {
			log.Printf("failed to get size pixel: %v", err)
		}
		log.Printf("size pixel %d,%d,%d,%d", x, y, z, v)
	}
	{
		x, y, err := stty.Size()
		if err != nil {
			log.Printf("failed to get size of raw stty: %v", err)
			return
		}
		log.Printf("width = %d, height = %d", x, y)
	}

	go func() {
		for v := range read {
			if _, err = stty.Output().Write(v); err != nil {
				log.Printf("failed to write to raw stty: %v", err.Error())
				return
			}
		}
	}()

	for {
		r, err := stty.ReadRune()
		if err != nil {
			log.Err(err).Msgf("failed to read rune")
			return
		}
		log.Printf("read rune: %v", string(r))
		write <- []byte(string(r))
	}
}

func readInput(ctx context.Context, c *websocket.Conn, write chan<- []byte, read <-chan []byte) {
	go func() {
		defer close(write)
		for {
			typ, buf, err := c.Read(ctx)
			if err != nil {
				log.Printf("failed to read: %v", err)
				return
			}
			fmt.Printf("type: %s, buf: %s\n", typ, buf)
			if typ == websocket.MessageText {
				write <- buf
			}
		}
	}()

	go func() {
		for {
			if err := c.Ping(ctx); err != nil {
				log.Printf("failed to ping: %v", err)
				return
			}
			time.Sleep(time.Second * 10)
		}
	}()

	for m := range read {
		if err := c.Write(ctx, websocket.MessageText, m); err != nil {
			log.Printf("failed to write: %v", err)
		}
	}
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
