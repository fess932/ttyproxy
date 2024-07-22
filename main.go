package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	err := run()
	if err != nil {
		log.Error().Err(err).Msg("failed to start server")
	}
}

//<serial type='pty'>
//<source path='/dev/pts/6'/>
//<target type='isa-serial' port='0'>
//<model name='isa-serial'/>
//</target>
//<alias name='serial0'/>
//</serial>

func run() error {
	l, err := net.Listen("tcp", "127.0.0.1:4444")
	if err != nil {
		return err
	}
	log.Printf("listening on ws://%v", l.Addr())

	p := &proxy{}
	s := &http.Server{
		Handler:      p,
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
	}

	errc := make(chan error, 1)
	go func() {
		errc <- s.Serve(l)
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	select {
	case err := <-errc:
		log.Printf("failed to serve: %v", err)
	case sig := <-sigs:
		log.Printf("terminating: %v", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()

	return s.Shutdown(ctx)
}
