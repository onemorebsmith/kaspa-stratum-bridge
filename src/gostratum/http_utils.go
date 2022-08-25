package gostratum

import (
	"bufio"
	"bytes"
	"log"
	"net"
	"strings"
	"time"

	"github.com/pkg/errors"
)

type LineCallback func(line string) error

func readFromConnection(connection net.Conn, cb LineCallback) error {
	deadline := time.Now().Add(5 * time.Second).UTC()
	log.Printf("blocking until %s", deadline)
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
