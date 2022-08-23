package kaspastratum

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"math/big"
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

	jobs := GenerateJobHeader(header)
	expected := []uint64{2769687437080476293, 2642455852654975590, 16370749824715673019, 15145064868898544117}
	if d := cmp.Diff(expected, jobs); d != "" {
		t.Fatalf("jobs generated incorrectly: %s", d)
	}
	log.Printf("%+v", jobs)

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
