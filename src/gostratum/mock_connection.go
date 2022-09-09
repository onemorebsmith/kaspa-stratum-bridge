package gostratum

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type MockConnection struct {
	id      string
	lock    sync.Mutex // to prevent double closing of channel
	inChan  chan []byte
	outChan chan []byte
}

var channelCounter int32

func NewMockConnection() *MockConnection {
	return &MockConnection{
		id:      fmt.Sprintf("mc_%d", atomic.AddInt32(&channelCounter, 1)),
		lock:    sync.Mutex{},
		inChan:  make(chan []byte),
		outChan: make(chan []byte),
	}
}

func (mc *MockConnection) AsyncWriteTestDataToReadBuffer(s string) {
	go func() {
		mc.inChan <- []byte(s)
	}()
}

func (mc *MockConnection) ReadTestDataFromBuffer(handler func([]byte)) {
	read := <-mc.outChan
	handler(read)
}

func (mc *MockConnection) AsyncReadTestDataFromBuffer(handler func([]byte)) {
	go func() {
		read := <-mc.outChan
		handler(read)
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
	mc.lock.Lock()
	defer mc.lock.Unlock()
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
		mc.lock.Lock()
		defer mc.lock.Unlock()
		time.Sleep(time.Until(t))
		close(mc.inChan)
		mc.inChan = make(chan []byte)
	}()

	return nil
}

func (mc *MockConnection) SetWriteDeadline(t time.Time) error {
	go func() {
		mc.lock.Lock()
		defer mc.lock.Unlock()
		time.Sleep(time.Until(t))
		close(mc.outChan)
		mc.outChan = make(chan []byte)
	}()

	return nil
}
