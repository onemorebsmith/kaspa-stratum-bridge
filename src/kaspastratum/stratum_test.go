package kaspastratum

import (
	"encoding/binary"
	"encoding/json"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"strings"
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
		expected := "266fe80cb20b3a8524abe3990026da66e330963ac8f74dbbd22e14419af219f59abb1a6300000000"
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
	log.Println(shareValue)
	log.Println(fixedDifficulty)
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

func TestBznonce(t *testing.T) {
	//noncestr := "0000000131468d2b" // orig
	noncestr := "0938a48b696f5186"

	//binary.LittleEndian.Uint64([]byte(noncestr)) // 3138241691631222784
	binary.LittleEndian.Uint64([]byte(noncestr)) // 3138241691631222784
	nonce := big.Int{}
	//noncestr = fmt.Sprintf("%x", noncestr)
	log.Printf("%x", binary.LittleEndian.Uint64([]byte(noncestr)))
	nonce.SetString(noncestr, 16)

	lolstr := "0x0085bad1a2de2d41"
	lolstr = strings.Replace(lolstr, "0x", "", 1)
	lolnonce := big.Int{}
	lolnonce.SetString(lolstr, 16)

	log.Printf("lol:\t%s", lolnonce.String())
	log.Printf("bz: \t%s", nonce.String())
}
6238346138333930
