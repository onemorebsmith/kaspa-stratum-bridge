package gostratum

import (
	"bufio"
	"bytes"
	"net"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func spawnClientListener(ctx *StratumContext, connection net.Conn, s *StratumListener) error {
	defer ctx.Disconnect()

	for {
		err := readFromConnection(connection, func(line string) error {
			event, err := UnmarshalEvent(line)
			if err != nil {
				ctx.Logger.Error("error unmarshalling event", zap.String("raw", line))
				return err
			}
			return s.HandleEvent(ctx, event)
		})
		if errors.Is(err, os.ErrDeadlineExceeded) {
			continue // expected timeout
		}
		if ctx.Err() != nil {
			return ctx.Err() // context cancelled
		}
		if ctx.parentContext.Err() != nil {
			return ctx.parentContext.Err() // parent context cancelled
		}
		if err != nil { // actual error
			ctx.Logger.Error("error reading from socket", zap.Error(err))
			return err
		}
	}
}

type LineCallback func(line string) error

func readFromConnection(connection net.Conn, cb LineCallback) error {
	deadline := time.Now().Add(5 * time.Second).UTC()
	if err := connection.SetReadDeadline(deadline); err != nil {
		return err
	}

	buffer := make([]byte, 1024)
	_, err := connection.Read(buffer)
	if err != nil {
		return errors.Wrapf(err, "error reading from connection")
	}
	buffer = bytes.ReplaceAll(buffer, []byte("\x00"), nil)
	scanner := bufio.NewScanner(strings.NewReader(string(buffer)))
	for scanner.Scan() {
		if err := cb(scanner.Text()); err != nil {
			return err
		}
	}
	return nil
}
