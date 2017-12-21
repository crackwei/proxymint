package proxy

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	//"encoding/hex"
	"errors"
	"math/big"
	"time"

	"github.com/BTCChina/mining-pool-proxy/stratum"
)

// work
type Work struct {
	stratum.ResponseNotify

	Height   int
	Target   stratum.Uint256
	RawBlock []byte
	SWork    []byte
	N        int
	K        int
	At       time.Time

	Difficulty Difficulty
	Subsidy    float64

	// TODO: server here
	lastBlock string
}

type ShareStatus string

const (
	ShareInvalid ShareStatus = "invalid"
	ShareOK      ShareStatus = "ok"
	ShareBlock   ShareStatus = "block"
)

// Get the the share bits from submission

// check the proof of work
// Returns the difficulty, return error if invalid.
func (w *Work) Check(nTime uint32, noncePart1, noncePart2, solution []byte, shareTarget stratum.Uint256, dead bool) ShareStatus {
	buffer := BuildBlockHeader(w.Version, w.HashPrevBlock[:], w.HashMerkleRoot[:], w.HashReserved[:], w.NTime, w.NBits, noncePart1, noncePart2)

	result, _ := Validate(w.N, w.K, buffer.Bytes(), solution, shareTarget, w.Target)
	if result == ShareBlock {
		if dead {
			return result
		}

		_, _ = buffer.Write([]byte{0xfd, 0x40, 0x05})
		_, _ = buffer.Write(solution)

		// The buffer now contains the completed block header
		// Fill in the rest of the block
		_, _ = buffer.Write(w.RawBlock[buffer.Len():])
		// SubmitBlock func, params: (buffer.Bytes(), hash)
	}

	return result
}

type Difficulty float64

var POWLimit *big.Int

func init() {
	POWLimit, _ = new(big.Int).SetString("0x00000000FFFF0000000000000000000000000000000000000000000000000000", 0)
}

func (d Difficulty) ToTarget() stratum.Uint256 {
	var result stratum.Uint256
	if d == 0 {
		copy(result[:], POWLimit.Bytes())
		return result
	}
	diff := big.NewInt(int64(d))

	bytes := new(big.Int).Div(POWLimit, diff).Bytes()

	if len(bytes) > len(result) {
		copy(result[:], POWLimit.Bytes())
		return result
	}

	copy(result[len(result)-len(bytes):], bytes)

	return result
}

func FromTarget(target stratum.Uint256) Difficulty {
	targ := target.ToInteger()
	if targ.Cmp(new(big.Int)) == 0 {
		// just return a vary large value
		return 10000000000000000000
	}

	res := new(big.Int).Div(POWLimit, targ)
	return Difficulty(res.Uint64())
}

func TargetCompare(a, b stratum.Uint256) int {
	return a.ToInteger().Cmp(b.ToInteger())
}

// CompactToTarget converts a NDiff value to a Target.
func CompactToTarget(x uint32) (stratum.Uint256, error) {
	x = reverseUint32(x)

	// The top byte is the number of bytes in the final string
	var result stratum.Uint256
	i := (x & 0xff000000) >> 24

	if int(i) > len(result) {
		return result, errors.New("invalid compact target")
	}

	parts := []byte{byte((x & 0x7f0000) >> 16), byte((x & 0xff00) >> 8), byte(x & 0xff)}
	copy(result[len(result)-int(i):], parts)

	return result, nil
}

// create block header
func BuildBlockHeader(version uint32, hashPrevBlock, hashMerkleRoot, hashReserved []byte, nTime, nBits uint32, noncePart1, noncePart2 []byte) *bytes.Buffer {
	buffer := bytes.NewBuffer(nil)
	_ = binary.Write(buffer, binary.BigEndian, version)
	_, _ = buffer.Write(hashPrevBlock)
	_, _ = buffer.Write(hashMerkleRoot)
	_, _ = buffer.Write(hashReserved)
	_ = binary.Write(buffer, binary.BigEndian, nTime)
	_ = binary.Write(buffer, binary.BigEndian, nBits)
	_, _ = buffer.Write(noncePart1)
	_, _ = buffer.Write(noncePart2)

	return buffer
}

// Validate checks POW validity of a header.
func Validate(n, k int, headerNonce []byte, solution []byte, shareTarget, globalTarget stratum.Uint256) (ShareStatus, string) {
	// ok, err := sha256.Verify(n, k, headerNonce, solution)
	// if err != nil {
	// 	return ShareInvalid, ""
	// }

	// if !ok {
	// 	return ShareInvalid, ""
	// }

	// // Double sha to check the target
	// hash := sha256.New()
	// _, _ = hash.Write(headerNonce)
	// _, _ = hash.Write([]byte{0xfd, 0x40, 0x05})
	// _, _ = hash.Write(solution)

	// round1 := hash.Sum(nil)
	// round2 := sha256.Sum256(round1[:])

	// // Reverse the hash
	// for i, j := 0, len(round2)-1; i < j; i, j = i+1, j-1 {
	// 	round2[i], round2[j] = round2[j], round2[i]
	// }

	// // Check against the global target
	// if TargetCompare(round2, globalTarget) <= 0 {
	// 	return ShareBlock, hex.EncodeToString(round2[:])
	// }

	// if TargetCompare(round2, shareTarget) > 1 {
	// 	return ShareInvalid, ""
	// }

	return ShareOK, ""
}

// Create merkle root

// Hashrate, accepts per minute, difficulty conversion functions
func ah2d(h, a float64) float64 {
	return (15 * h) / (1073741824 * a)
}

func hd2a(h, d float64) float64 {
	return (15 * h) / (1073741824 * d)
}

func ad2h(a, d float64) float64 {
	return (1073741824 * a * d) / 15
}

// Check if the miner's address is valid
func IsValidAddress(addr string) (valid bool, testnet bool) {
	address, err := DecodeCheck(addr)
	if err != nil {
		return false, false
	}

	switch len(address) {
	case 1 + 1 + 20: // Testnet
		if address[0] == 0x1c && (address[1] == 0xbd || address[1] == 0xb8) {
			return true, false
		}
		if address[0] == 0x1d && (address[1] == 0xba || address[1] == 0x25) {
			return true, true
		}
	case 1 + 1 + 32 + 32: // Production
		if address[0] == 0x16 {
			if address[1] == 0x9a {
				return true, false
			} else if address[1] == 0xb6 {
				return true, true
			}
		}
	default:
		return false, false
	}

	return false, false
}

// Verifies the validity of a base58 encoded address using the checksum
var decoder = map[byte]int64{
	'1': 0,
	'2': 1,
	'3': 2,
	'4': 3,
	'5': 4,
	'6': 5,
	'7': 6,
	'8': 7,
	'9': 8,
	'A': 9,
	'B': 10,
	'C': 11,
	'D': 12,
	'E': 13,
	'F': 14,
	'G': 15,
	'H': 16,
	'J': 17,
	'K': 18,
	'L': 19,
	'M': 20,
	'N': 21,
	'P': 22,
	'Q': 23,
	'R': 24,
	'S': 25,
	'T': 26,
	'U': 27,
	'V': 28,
	'W': 29,
	'X': 30,
	'Y': 31,
	'Z': 32,
	'a': 33,
	'b': 34,
	'c': 35,
	'd': 36,
	'e': 37,
	'f': 38,
	'g': 39,
	'h': 40,
	'i': 41,
	'j': 42,
	'k': 43,
	'm': 44,
	'n': 45,
	'o': 46,
	'p': 47,
	'q': 48,
	'r': 49,
	's': 50,
	't': 51,
	'u': 52,
	'v': 53,
	'w': 54,
	'x': 55,
	'y': 56,
	'z': 57,
}

// DecodeCheck decodes a base58check encoded address.
func DecodeCheck(str string) ([]byte, error) {
	leading := true
	var leadingZeros int

	// Run the base58 decode
	x := big.NewInt(0)
	base := big.NewInt(58)
	for _, v := range []byte(str) {
		y, ok := decoder[v]
		if !ok {
			return nil, errors.New("base58: invalid character detected")
		}

		if y == 0 {
			if leading {
				leadingZeros++
			}
		} else {
			leading = false
		}

		x.Mul(x, base)
		x.Add(x, big.NewInt(y))
	}

	var data []byte
	for i := 0; i < leadingZeros; i++ {
		data = append(data, 0)
	}

	data = append(data, x.Bytes()...)
	if len(data) < 4 {
		return nil, errors.New("base58: too short")
	}

	raw := data[:len(data)-4]
	checksum := data[len(data)-4:]

	result1 := sha256.Sum256(raw)
	result2 := sha256.Sum256(result1[:])

	if !bytes.Equal(checksum, result2[:4]) {
		return nil, errors.New("base58: base checksum")
	}

	return raw, nil
}

func reverseUint32(x uint32) uint32 {
	return (uint32(x)&0xff000000)>>24 |
		(uint32(x)&0x00ff0000)>>8 |
		(uint32(x)&0x0000ff00)<<8 |
		(uint32(x)&0x000000ff)<<24
}
