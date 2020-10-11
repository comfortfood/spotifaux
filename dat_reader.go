package spotifaux

import (
	"encoding/binary"
	"os"
)

type datReader struct {
	f      *os.File
	Frames int
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
	r.Frames = int(binary.LittleEndian.Uint64(fb))

	return r, nil
}

func (r *datReader) Dat() ([]float64, error) {
	b := make([]uint8, r.cqtN)
	_, err := r.f.Read(b)
	if err != nil {
		return nil, err
	}

	features := make([]float64, r.cqtN)

	for i := 0; i < r.cqtN; i++ {
		features[i] = (float64(b[i]) - 128.0) / 255.0
	}

	return features, nil
}

func (r *datReader) Close() error {
	return r.f.Close()
}
