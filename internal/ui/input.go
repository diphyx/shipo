package ui

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"unicode/utf8"
)

// readChunks starts the one reader of the input. It hands out what it reads
// as it comes, so a prompt can take a line, or a single key in raw mode, and
// it closes chunks at the end of input.
func (terminal *Terminal) readChunks() {
	terminal.chunks = make(chan []byte)

	go func() {
		defer close(terminal.chunks)

		buffer := make([]byte, 256)

		for {
			count, readError := terminal.input.Read(buffer)
			if count > 0 {
				terminal.chunks <- append([]byte(nil), buffer[:count]...)
			}

			if readError != nil {
				return
			}
		}
	}()
}

// fill waits for more input. It returns ErrCancelled when the context is
// cancelled, as Ctrl-C does, or at the end of input.
func (terminal *Terminal) fill(context context.Context) error {
	terminal.readOnce.Do(terminal.readChunks)

	select {
	case <-context.Done():
		{
			return ErrCancelled
		}
	case chunk, isOpen := <-terminal.chunks:
		{
			if !isOpen {
				terminal.isEnded = true

				return ErrCancelled
			}

			terminal.pending = append(terminal.pending, chunk...)

			return nil
		}
	}
}

// readLine reads one answer, up to a newline.
func (terminal *Terminal) readLine(context context.Context) (string, error) {
	for {
		end := bytes.IndexByte(terminal.pending, '\n')
		if end >= 0 {
			line := strings.TrimSpace(string(terminal.pending[:end]))
			terminal.pending = terminal.pending[end+1:]

			return line, nil
		}

		fillError := terminal.fill(context)
		if fillError == nil {
			continue
		}

		if context.Err() != nil {
			_, _ = fmt.Fprintln(terminal.Out)
		}

		if terminal.isEnded && len(terminal.pending) > 0 {
			line := strings.TrimSpace(string(terminal.pending))
			terminal.pending = nil

			return line, nil
		}

		return "", fillError
	}
}

// readKey reads one key in raw mode: an arrow as its escape sequence, or one
// character.
func (terminal *Terminal) readKey(context context.Context) (string, error) {
	for len(terminal.pending) == 0 {
		fillError := terminal.fill(context)
		if fillError != nil {
			return "", fillError
		}
	}

	size := 1

	switch {
	case bytes.HasPrefix(terminal.pending, []byte("\x1b[")) && len(terminal.pending) >= 3:
		{
			size = 3
		}
	case terminal.pending[0] >= utf8.RuneSelf:
		{
			_, size = utf8.DecodeRune(terminal.pending)
		}
	}

	key := string(terminal.pending[:size])
	terminal.pending = terminal.pending[size:]

	return key, nil
}
