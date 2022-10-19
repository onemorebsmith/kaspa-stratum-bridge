package kaspastratum

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/kaspanet/kaspad/app/appmessage"
	"github.com/kaspanet/kaspad/util/difficulty"
)

func TestHeaderSerialization(t *testing.T) {
	raw, err := ioutil.ReadFile("./example_header.json")
	if err != nil {
		t.Fatal(err)
	}
	block := appmessage.RPCBlock{}
	if err := json.Unmarshal(raw, &block.Header); err != nil {
		t.Fatal(err)
	}

	header, err := SerializeBlockHeader(&block)
	if err != nil {
		t.Fatal(err)
	}
	headerExpected := []byte{133, 58, 11, 178, 12, 232, 111, 38, 102, 218, 38, 0, 153, 227, 171, 36, 187, 77, 247, 200, 58, 150, 48, 227, 245, 25, 242, 154, 65, 20, 46, 210}
	if d := cmp.Diff(headerExpected, header); d != "" {
		t.Fatalf("header generated incorrectly: %s", d)
	}

	{ // job separated to parts (lolminer/srbminer)
		jobs := GenerateJobHeader(header)
		expected := []uint64{2769687437080476293, 2642455852654975590, 16370749824715673019, 15145064868898544117}
		if d := cmp.Diff(expected, jobs); d != "" {
			t.Fatalf("jobs generated incorrectly: %s", d)
		}
		log.Printf("%+v", jobs)
	}

	{ // job as single string (bzminer)
		job := GenerateLargeJobParams(header, 1662696346)
		expected := "853a0bb20ce86f2666da260099e3ab24bb4df7c83a9630e3f519f29a41142ed29abb1a6300000000"
		if d := cmp.Diff(expected, job); d != "" {
			t.Fatalf("jobs generated incorrectly: %s", d)
		}
		log.Printf("%+v", job)
	}

	// expected diff: 12617.375671633985 (approx)
	diff := CalculateTarget(453325233)
	little := BigDiffToLittle(&diff)
	if little < 12617 || little > 12618 {
		t.Errorf("wrong difficulty calculated, expected ~12617.375671633985, got %f", little)
	}
}

func TestPoolHzCalculation(t *testing.T) {
	// TODO: figure out what we really want to test here.
	// currently set up diff object to mimic old static settings
	diff := newKaspaDiff()
	diff.setDiffValue(4)
	log.Println(diff.hashValue)
	log.Println(diff.diffValue)
	rate := big.Int{} // 1mhz
	rate.SetUint64(1)
	rate.Lsh(&rate, 222)
	dd := BigDiffToLittle(&rate)
	log.Println(dd)
	//rate.Sub(&rate, big.NewInt(1000000))
	log.Println(difficulty.GetHashrateString(&rate, time.Second*1))
}

// snooper. Inspect coms between miner and pool
func TestBridge(t *testing.T) {
	serverConn, err := net.Dial("tcp", "pool.us.woolypooly.com:3112")
	if err != nil {
		t.Fatal(err)
	}
	clientConn, err := net.Listen("tcp", ":4444")
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
