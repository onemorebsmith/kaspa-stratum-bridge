package gostratum

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"testing"
	"time"

	"github.com/kaspanet/kaspad/app/appmessage"
)

func TestServer(t *testing.T) {
	cfg := BridgeConfig{
		RPCServer:  "localhost:16110",
		MiningAddr: "kaspa:qzk3uh2twkhu0fmuq50mdy3r2yzuwqvstq745hxs7tet25hfd4egcafcdmpdl",
	}
	if _, err := ListenAndServe(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestBridge(t *testing.T) {
	serverConn, err := net.Dial("tcp", "pool.us.woolypooly.com:3112")
	if err != nil {
		t.Fatal(err)
	}
	clientConn, err := net.Listen("tcp", ":8080")
	if err != nil {
		t.Fatal(err)
	}
	c, _ := clientConn.Accept()
	buff := make([]byte, 1024)
	for {
		time.Sleep(100 * time.Millisecond)
		bytesRead, _ := c.Read(buff)
		log.Printf("client -> server: %d, '%s'", bytesRead, string(buff))
		serverConn.Write(buff)
		bytesRead, _ = serverConn.Read(buff)
		log.Printf("server -> client: %d, '%s'", bytesRead, string(buff))
		c.Write(buff)
	}
}

func TestHeaderSerialization(t *testing.T) {
	raw, err := ioutil.ReadFile("./example_header.json")
	if err != nil {
		t.Fatal(err)
	}
	block := appmessage.RPCBlock{}
	if err := json.Unmarshal(raw, &block.Header); err != nil {
		t.Fatal(err)
	}

	res, err := SerializeBlockHeader(&block)
	if err != nil {
		t.Fatal(err)
	}
	log.Println(res)
	jobs := GenerateJobHeader(res)
	log.Printf("%+v", jobs)

	// expected diff: 12617.375671633985 (approx)
	diff := CalculateTarget(453325233)
	if diff < 12617 || diff > 12618 {
		t.Errorf("wrong difficulty calculated, expected ~12617.375671633985, got %f", diff)
	}
}
