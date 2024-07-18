package main

import (
	"bufio"
	"context"
	"fmt"
	"golang.org/x/term"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog/log"

	gotty "github.com/mattn/go-tty"
	"nhooyr.io/websocket"
)

type rw struct {
	clientReader io.Reader
	clientWriter io.Writer
	tty          *os.File
}

func (r *rw) Read(p []byte) (n int, err error) {
	log.Printf("READ START %d", len(p))
	n, err = r.tty.Read(p)
	log.Printf("READ END!!! len %d, %d, %v: %v", len(p), n, p, err)
	return n, err
}
func (r *rw) Write(p []byte) (n int, err error) {
	//log.Printf("WRITE START")
	//if len(p) == 0 {
	//	return 0, nil
	//}
	n, err = r.tty.Write(p)
	log.Printf("WRITE!!! %d: %v,%v", len(p), p, err)
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
	//tty, err := os.OpenFile("/dev"+ptyName, os.O_RDWR, os.ModePerm)
	//if err != nil {
	//	log.Err(err).Msgf("failed to open tty %s", ptyName)
	//	return
	//}
	//fd := int(tty.Fd())
	//if !term.IsTerminal(fd) {
	//	log.Error().Msgf("%v is not a terminal", ptyName)
	//}
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
			log.Printf("failed to backup backup: %v", err)
		}
	}()
	go func() {
		ch := stty.SIGWINCH()
		log.Printf("chan start")
		for v := range ch {
			log.Printf("channel! %v %v", v.H, v.W)
		}
	}()
	{
		x, y, z, v, err := stty.SizePixel()
		if err != nil {
			log.Printf("failed to get size pixel: %v", err)
		}
		log.Printf("size pixel %d,%d,%d,%d", x, y, z, v)
	}

	x, y, err := stty.Size()
	if err != nil {
		log.Printf("failed to get size of raw stty: %v", err)
		return
	}
	log.Printf("width = %d, height = %d", x, y)

	//if _, err = stty.Output().WriteString("ls"); err != nil {
	//	log.Printf("failed to write to raw stty: %v", err.Error())
	//	return
	//}

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

	str, err := stty.ReadString()
	if err != nil {
		log.Err(err).Msgf("failed to read stty str")
		return
	}
	fmt.Printf("String:' %s ' \n", str)

	for i := 0; i < y; i++ {
		str, err := stty.ReadString()
		if err != nil {
			log.Err(err).Msgf("failed to read stty str")
			return
		}
		fmt.Printf("String:' %s ' \n", str)
	}
	return

	//oldState, err := term.MakeRaw(fd)
	//if err != nil {
	//	panic(err)
	//}
	//defer term.Restore(fd, oldState)
	//
	//log.Debug().Msgf("Websocket connection init to: %d", fd)
	//

	//
	////if c.Subprotocol() != "echo" {
	////	c.Close(websocket.StatusPolicyViolation, "client must speak the echo subprotocol")
	////	log.Err(err).Msgf("client must speak the echo subprotocol")
	////	return
	////}
	//
	////rwriter := &rw{tty: tty}
	////rwriter := &rw{tty: tty}
	//terminal := term.NewTerminal(tty, "")
	//ls := []byte("ls")
	//terminal.Write(ls)
	//
	//for i := 0; i < 24; i++ {
	//	line, err := terminal.ReadLine()
	//	time.Sleep(time.Millisecond * 60)
	//	log.Printf("Err: %v, LINE: ' %s '", err, line)
	//}
	//
	//return
	//
	//log.Printf("new iteration")
	//err = handlePty(r.Context(), c, nil, tty)
	//if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
	//	log.Printf("connection closed normally")
	//	return
	//}
	//
	//if err != nil {
	//	log.Printf("failed to echo with %v: %v", r.RemoteAddr, err)
	//	return
	//}
	//
	//log.Printf("Connection closed")
}

func handlePty(ctx context.Context, c *websocket.Conn, terminal *term.Terminal, tty *os.File) error {
	typ, r, err := c.Reader(ctx)
	if err != nil {
		return fmt.Errorf("failed to get reader")
	}
	log.Printf("reader type: %s", typ)

	w, err := c.Writer(ctx, websocket.MessageText)
	if err != nil {
		return fmt.Errorf("faield to get writer")
	}
	defer w.Close()
	rwriter := &rw{tty: tty}
	terminal = term.NewTerminal(rwriter, "")
	// set size for terminal
	// terminal.SetSize()

	reader := bufio.NewReader(r)
	for {
		str, _, err := reader.ReadLine()
		if err == io.EOF {
			time.Sleep(time.Millisecond * 60)
			continue
		}
		if err != nil {
			log.Printf("failed to read line: %v", err)
		}
		log.Printf("LINE: %s", str)
	}

	//log.Printf("copy from client to tty")
	//if _, err = io.Copy(terminal, r); err != nil {
	//	log.Printf("failed to copy to tty")
	//}
	//log.Printf("end copy from client to tty")

	//go func() {
	//	for {
	//		time.Sleep(io.Copy())
	//	}
	//}()
	ls := []byte("ls")
	terminal.Write(ls)
	line, err := terminal.ReadLine()
	log.Printf("LINE: ' %s '", line)
	return nil

	//go func() {
	//	for {
	//		if _, err = io.Copy(terminal, r); err != nil {
	//			log.Printf("go error: %v", err)
	//		}
	//	}
	//}()

	for {
		line, err := terminal.ReadLine()
		if err != nil {
			if err == io.EOF {
				log.Printf("got EOF")
			} else {
				log.Printf("got err: %v", err)
			}
		}
		if line == "" {
			continue
		}
		log.Printf("LINE: %s", line)
		in, err := w.Write([]byte(line))
		if err != nil {
			log.Printf("failed to write: %v", err)
		}
		log.Printf("writed %d", in)
	}

	var i int64
	for {
		if i, err = io.Copy(terminal, r); err != nil {
			log.Printf("failed to copy to tty")
		}
		log.Printf("copied %d", i)
		time.Sleep(time.Second * 1)

		var line string
		if line, err = terminal.ReadLine(); err != nil {
			log.Printf("failed to read line: %v", err.Error())
		} else {
			fmt.Println(line)

			var in int
			in, err = w.Write([]byte(line))
			log.Printf("wrote %d", in)
		}
		//for {
		//	time.Sleep(time.Second)
		//	line, err := terminal.ReadLine()
		//	log.Printf("line from tty: %s", line)
		//	if err == io.EOF {
		//		break
		//	}
		//	if err != nil {
		//		log.Printf("failed to read line from tty")
		//		break
		//	}
		//	w.Write([]byte(line))
		//}

		//if _, err = io.Copy(w, terminal); err != nil {
		//	log.Printf("failed to copy to tty")
		//}

	}

	//println(1)
	//go func() {
	//	if _, err = io.Copy(tty, r); err != nil {
	//		log.Err(err).Msgf("failed to copy to terminal")
	//	}
	//}()
	//
	//if _, err = io.Copy(w, tty); err != nil {
	//	log.Err(err).Msgf("failed to copy from terminal")
	//}
	//
	//println(3)
	////log.Printf("tgermianl! %+v", terminal)
	//line, err := terminal.ReadLine()
	//if err != nil {
	//	log.Error().Msgf("failed to read line: %v", err)
	//}
	//w.Write([]byte(line))
	//println("line:", line)

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

	return nil
}

func readInput(ctx context.Context, c *websocket.Conn, write chan<- []byte, read <-chan []byte) {
	//typ, r, err := c.Reader(ctx)
	//if err != nil {
	//	log.Printf("failed to get reader: %v", err)
	//	return
	//}
	//w, err := c.Writer(ctx, typ)
	//if err != nil {
	//	log.Printf("failed to get writer: %v", err)
	//	return
	//}

	//read := make(chan []byte)

	go func() {
		//defer close(read)
		for {
			typ, buf, err := c.Read(ctx)
			if err != nil {
				log.Printf("failed to read: %v", err)
				return
			}
			fmt.Printf("type: %s, buf: %s\n", typ, buf)
			if typ == websocket.MessageText {
				//read <- buf
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
			time.Sleep(time.Second * 3)
		}
	}()

	for m := range read {
		if err := c.Write(ctx, websocket.MessageText, m); err != nil {
			log.Printf("failed to write: %v", err)
		}
		//
		//if _, err = io.Copy(os.Stdout, r); err != nil {
		//	log.Printf("failed to copy to stdout: %v", err)
		//	time.Sleep(time.Second)
		//	return
		//}
		//c.Ping(ctx)
		//if w.Write([]byte("ok"))
		//c.Ping(ctx)
	}

	//log.Printf("read input end")
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
