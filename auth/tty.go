package auth

import (
	"bytes"
	"io"
	"log"
	"sync"
)

type (
	TTY struct {
		dest   io.ReadWriter
		logger *log.Logger
		buf    bytes.Buffer
		mux    sync.Mutex
	}
)

func NewTTY(dest io.ReadWriter, prefix string, flag int) *TTY {
	tty := TTY{dest: dest}

	tty.logger = log.New(&tty.buf, prefix, flag)

	return &tty
}

func (t *TTY) Read(p []byte) (n int, err error) {
	t.mux.Lock()
	defer t.mux.Unlock()

	return t.dest.Read(p)
}

func (t *TTY) Write(p []byte) (n int, err error) {
	t.mux.Lock()
	defer t.mux.Unlock()

	return t.dest.Write(p)
}

func (t *TTY) Printf(format string, v ...any) {
	t.mux.Lock()
	defer t.mux.Unlock()

	t.logger.Printf(format, v...)
}

func (t *TTY) Print(v ...any) {
	t.mux.Lock()
	defer t.mux.Unlock()

	t.logger.Print(v...)
}

func (t *TTY) FlushLogs() error {
	t.mux.Lock()
	defer t.mux.Unlock()

	_, err := t.dest.Write(t.buf.Bytes())

	return err
}
