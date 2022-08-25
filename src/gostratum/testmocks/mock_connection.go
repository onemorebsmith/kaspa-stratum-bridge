package testmocks

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"time"
)

type MockConnection struct {
	id      string
	inChan  chan []byte
	outChan chan []byte
}

var channelCounter int32

func NewMockConnection() *MockConnection {
	return &MockConnection{
		id:      fmt.Sprintf("mc_%d", atomic.AddInt32(&channelCounter, 1)),
		inChan:  make(chan []byte),
		outChan: make(chan []byte),
	}
}

func (mc *MockConnection) AsyncWriteTestDataToReadBuffer(s string) {
	go func() {
		mc.inChan <- []byte(s)
	}()
}

func (mc *MockConnection) Read(b []byte) (int, error) {
	data, ok := <-mc.inChan
	if !ok {
		return 0, context.DeadlineExceeded
	}
	return copy(b, data), nil
}

func (mc *MockConnection) Write(b []byte) (int, error) {
	mc.outChan <- b
	return len(b), nil
}

func (mc *MockConnection) Close() error {
	close(mc.inChan)
	close(mc.outChan)
	return nil
}

type MockAddr struct {
	id string
}

func (ma MockAddr) Network() string { return "mock" }
func (ma MockAddr) String() string  { return ma.id }

func (mc *MockConnection) LocalAddr() net.Addr {
	return MockAddr{id: mc.id}
}

func (mc *MockConnection) RemoteAddr() net.Addr {
	return MockAddr{id: mc.id}
}

func (mc *MockConnection) SetDeadline(t time.Time) error {
	mc.SetReadDeadline(t)
	mc.SetWriteDeadline(t)
	return nil
}

func (mc *MockConnection) SetReadDeadline(t time.Time) error {
	go func() {
		time.Sleep(time.Until(t))
		close(mc.inChan)
		mc.inChan = make(chan []byte)
	}()

	return nil
}

func (mc *MockConnection) SetWriteDeadline(t time.Time) error {
	go func() {
		time.Sleep(time.Until(t))
		close(mc.outChan)
		mc.outChan = make(chan []byte)
	}()

	return nil
}