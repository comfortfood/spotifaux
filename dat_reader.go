package spotifaux

import (
	"encoding/binary"
	"math"
	"os"
)

type datReader struct {
	f      *os.File
	frames int
	cqtN   int
}

func NewDatReader(fileName string, cqtN int) (*datReader, error) {
	r := &datReader{
		cqtN: cqtN,
	}

	f, err := os.Open(fileName)
	if err != nil {
		panic(err)
	}
	r.f = f

	fb := make([]byte, 8)
	_, err = r.f.Read(fb)
	if err != nil {
		return nil, err
	}
	r.frames = int(binary.LittleEndian.Uint64(fb))

	return r, nil
}

func (r *datReader) Dat() ([]float64, error) {
	b := make([]byte, 8*r.cqtN)
	_, err := r.f.Read(b)
	if err != nil {
		return nil, err
	}

	features := make([]float64, r.cqtN)

	for i := 0; i < r.cqtN; i++ {
		features[i] = math.Float64frombits(binary.LittleEndian.Uint64(b[8*i : 8+8*i]))
	}

	return features, nil
}

func (r *datReader) Close() error {
	return r.f.Close()
}
