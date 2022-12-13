package kaspastratum

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"log"
	"math/big"

	"github.com/kaspanet/kaspad/app/appmessage"
	"golang.org/x/crypto/blake2b"
)

// static value definitions to avoid overhead in diff translations
var (
	maxTarget = big.NewFloat(0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF)
	minHash   = new(big.Float).Quo(new(big.Float).SetMantExp(big.NewFloat(1), 256), maxTarget)
	bigGig    = big.NewFloat(1e9)
)

// Basically three different ways of representing difficulty, each used on
// different occasions.  All 3 are updated when the stratum diff is set via 
// the setDiffValue method
type kaspaDiff struct {
	hashValue   float64  // previously known as shareValue
	diffValue   float64  // previously known as fixedDifficulty
	targetValue *big.Int // previously know as fixedDifficultyBI
}

func newKaspaDiff() *kaspaDiff {
	return &kaspaDiff{}
}

func (k *kaspaDiff) setDiffValue(diff float64) {
	k.diffValue = diff
	k.targetValue = DiffToTarget(diff)
	k.hashValue = DiffToHash(diff)
}

func DiffToTarget(diff float64) *big.Int {
	target := new(big.Float).Quo(maxTarget, big.NewFloat(diff))

	t, _ := target.Int(nil)
	return t
}

func DiffToHash(diff float64) float64 {
	hashVal := new(big.Float).Mul(minHash, big.NewFloat(diff))
	hashVal.Quo(hashVal, bigGig)

	h, _ := hashVal.Float64()
	return h
}

func SerializeBlockHeader(template *appmessage.RPCBlock) ([]byte, error) {
	hasher, err := blake2b.New(32, []byte("BlockHash"))
	if err != nil {
		return nil, err
	}
	write16(hasher, uint16(template.Header.Version))
	write64(hasher, uint64(len(template.Header.Parents)))
	for _, v := range template.Header.Parents {
		write64(hasher, uint64(len(v.ParentHashes)))
		for _, hash := range v.ParentHashes {
			writeHexString(hasher, hash)
		}
	}
	writeHexString(hasher, template.Header.HashMerkleRoot)
	writeHexString(hasher, template.Header.AcceptedIDMerkleRoot)
	writeHexString(hasher, template.Header.UTXOCommitment)

	// pack the rest of the header at once
	data := struct {
		TS        uint64
		Bits      uint32
		Nonce     uint64
		DAAScore  uint64
		BlueScore uint64
	}{
		TS:        uint64(0),
		Bits:      uint32(template.Header.Bits),
		Nonce:     uint64(0),
		DAAScore:  uint64(template.Header.DAAScore),
		BlueScore: uint64(template.Header.BlueScore),
	}

	detailsBuff := &bytes.Buffer{}
	if err := binary.Write(detailsBuff, binary.LittleEndian, data); err != nil {
		return nil, err
	}
	hasher.Write(detailsBuff.Bytes())

	bw := template.Header.BlueWork
	padding := len(bw) + (len(bw) % 2)
	for {
		if len(bw) < padding {
			bw = "0" + bw
		} else {
			break
		}
	}
	hh, _ := hex.DecodeString(bw)
	write64(hasher, uint64(len(hh)))
	writeHexString(hasher, bw)
	writeHexString(hasher, template.Header.PruningPoint)

	final := hasher.Sum(nil)
	//log.Println(final)
	return final, nil
}

func GenerateJobHeader(headerData []byte) []uint64 {
	ids := []uint64{}
	ids = append(ids, uint64(binary.LittleEndian.Uint64(headerData[0:])))
	ids = append(ids, uint64(binary.LittleEndian.Uint64(headerData[8:])))
	ids = append(ids, uint64(binary.LittleEndian.Uint64(headerData[16:])))
	ids = append(ids, uint64(binary.LittleEndian.Uint64(headerData[24:])))

	final := []uint64{}
	for _, v := range ids {
		asHex := fmt.Sprintf("%x", v)
		bb := big.Int{}
		bb.SetString(asHex, 16)

		final = append(final, bb.Uint64())
	}
	return final
}

func GenerateLargeJobParams(headerData []byte, timestamp uint64) string {
	ids := []uint64{}
	timestampBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(timestampBytes, timestamp)
	ids = append(ids, uint64(binary.BigEndian.Uint64(headerData[0:])))
	ids = append(ids, uint64(binary.BigEndian.Uint64(headerData[8:])))
	ids = append(ids, uint64(binary.BigEndian.Uint64(headerData[16:])))
	ids = append(ids, uint64(binary.BigEndian.Uint64(headerData[24:])))
	ids = append(ids, uint64(binary.LittleEndian.Uint64(timestampBytes)))

	str := fmt.Sprintf("%016x%016x%016x%016x%016x", ids[0], ids[1], ids[2], ids[3], ids[4])
	if len(str) != 80 {
		for _, v := range ids {
			log.Printf("%016x", v)
		}
		log.Printf("%s : %d", str, len(str))
	}
	return str
}

var bi = big.NewInt(16777215)

func CalculateTarget(bits uint64) big.Int {
	truncated := uint64(bits) >> 24
	mantissa := bits & bi.Uint64()
	exponent := uint64(0)
	if truncated <= 3 {
		mantissa = mantissa >> (8 * (3 - truncated))
	} else {
		exponent = 8 * ((bits >> 24) - 3)
	}

	// actual final diff (mant << exp)
	diff := big.Int{}
	diff.SetUint64(mantissa)
	diff.Lsh(&diff, uint(exponent))

	return diff
}

func BigDiffToLittle(diff *big.Int) float64 {
	// this is constant
	numerator := &big.Int{}
	numerator.SetUint64(2)
	numerator.Lsh(numerator, 254)

	final := big.Float{}
	final.SetInt(numerator)
	tempA := big.Float{}
	tempA.SetInt(diff)
	final = *final.Quo(&final, &tempA)

	tempA.SetInt64(2 << 30)
	final = *final.Quo(&final, &tempA)
	d, _ := final.Float64()
	return d
}

func write16(hasher hash.Hash, val uint16) {
	intBuff := make([]byte, 2)
	binary.LittleEndian.PutUint16(intBuff, val)
	hasher.Write(intBuff)
}

func write64(hasher hash.Hash, val uint64) {
	intBuff := make([]byte, 8)
	binary.LittleEndian.PutUint64(intBuff, val)
	hasher.Write(intBuff)
}

func writeHexString(hasher hash.Hash, val string) {
	hexBw, _ := hex.DecodeString(val)
	hasher.Write([]byte(hexBw))
}
