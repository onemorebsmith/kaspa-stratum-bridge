package utils

import (
	"bufio"
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/multierr"
	"go.uber.org/zap/zapcore"
)

const (
	// _defaultBufferSize specifies the default size used by Buffer.
	_defaultBufferSize = 256 * 1024 // 256 kB

	// _defaultFlushInterval specifies the default flush interval for
	// Buffer.
	_defaultFlushInterval = 30 * time.Second
)

var ErrWSFlush = errors.New("unable to flush write stream")

// buffers writes in-memory before flushing them to a wrapped WriteSyncer after
// reaching some limit, or at some fixed interval--whichever comes first.
type BufferedWriteSyncer struct {
	// WS is the WriteSyncer around which BufferedWriteSyncer will buffer
	// writes.
	//
	// This field is required.
	WS zapcore.WriteSyncer

	// Size specifies the maximum amount of data the writer will buffered
	// before flushing.
	//
	// Defaults to 256 kB if unspecified.
	Size int

	// FlushInterval specifies how often the writer should flush data if
	// there have been no writes.
	//
	// Defaults to 30 seconds if unspecified.
	FlushInterval time.Duration

	// Clock, if specified, provides control of the source of time for the
	// writer.
	//
	// Defaults to the system clock.
	Clock zapcore.Clock

	// unexported fields for state
	mu          sync.Mutex
	initialized bool // whether initialize() has run
	stopped     bool // whether Stop() has run
	writer      *bufio.Writer
	ticker      *time.Ticker
	stop        chan struct{} // closed when flushLoop should stop
	done        chan struct{} // closed when flushLoop has stopped
}

func (s *BufferedWriteSyncer) initialize() {
	size := s.Size
	if size == 0 {
		size = _defaultBufferSize
	}

	flushInterval := s.FlushInterval
	if flushInterval == 0 {
		flushInterval = _defaultFlushInterval
	}

	if s.Clock == nil {
		s.Clock = zapcore.DefaultClock
	}

	s.ticker = s.Clock.NewTicker(flushInterval)
	s.writer = bufio.NewWriterSize(s.WS, size)
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	s.initialized = true
	go s.flushLoop()
}

// Write writes log data into buffer syncer directly, multiple Write calls will
// be batched, and log data will be flushed to disk when the buffer is full or
// periodically.
func (s *BufferedWriteSyncer) Write(bs []byte) (int, error) {
	locked := false
	tryCount := 0
	for {
		locked = s.mu.TryLock()
		if !locked {
			if tryCount < 5 {
				time.Sleep(100 * time.Millisecond)
				tryCount++
			} else {
				// silently dropping log messages if we can't acquire lock
				return len(bs), nil
			}
		} else {
			break
		}
	}

	defer s.mu.Unlock()

	if !s.initialized {
		s.initialize()
	}

	// To avoid partial writes from being flushed, we manually flush the
	// existing buffer if:
	// * The current write doesn't fit into the buffer fully, and
	// * The buffer is not empty (since bufio will not split large writes when the buffer is empty)
	if len(bs) > s.writer.Available() && s.writer.Buffered() > 0 {
		err := s.Flush()
		if err != nil {
			if errors.Is(err, ErrWSFlush) {
				// silently dropping log messages if we can't flush buffer
				return len(bs), nil
			} else {
				return 0, err
			}
		}
	}

	return s.writer.Write(bs)
}

// Flush with timeout
func (s *BufferedWriteSyncer) Flush() error {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	c := make(chan error)

	go func(ctx context.Context) {
		err := s.writer.Flush()
		select {
		case <-ctx.Done():
			c <- ErrWSFlush
		default:
			c <- err
		}
	}(ctx)

	return <-c
}

// Sync flushes buffered log data into disk directly.
func (s *BufferedWriteSyncer) Sync() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error
	if s.initialized {
		err = s.Flush()
	}

	if errors.Is(err, ErrWSFlush) {
		// unable to flush buffer, fail silently and skip WS sync
		return nil
	} else {
		return multierr.Append(err, s.WS.Sync())
	}
}

// flushLoop flushes the buffer at the configured interval until Stop is
// called.
func (s *BufferedWriteSyncer) flushLoop() {
	defer close(s.done)

	for {
		select {
		case <-s.ticker.C:
			// we just simply ignore error here
			// because the underlying bufio writer stores any errors
			// and we return any error from Sync() as part of the close
			_ = s.Sync()
		case <-s.stop:
			return
		}
	}
}

// Stop closes the buffer, cleans up background goroutines, and flushes
// remaining unwritten data.
func (s *BufferedWriteSyncer) Stop() (err error) {
	var stopped bool

	// Critical section.
	func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		if !s.initialized {
			return
		}

		stopped = s.stopped
		if stopped {
			return
		}
		s.stopped = true

		s.ticker.Stop()
		close(s.stop) // tell flushLoop to stop
		<-s.done      // and wait until it has
	}()

	// Don't call Sync on consecutive Stops.
	if !stopped {
		err = s.Sync()
	}

	return err
}
