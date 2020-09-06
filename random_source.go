package main

import (
	"bufio"
	"os"
	"strconv"
)

type randomSource struct {
	f       *os.File
	scanner *bufio.Scanner
}

func newRandomSource() randomSource {
	r := randomSource{}

	f, err := os.Open("/Users/wyatttall/git/BLAST/soundspotter/out")
	if err != nil {
		panic(err)
	}
	r.f = f

	r.scanner = bufio.NewScanner(f)
	r.scanner.Split(bufio.ScanWords)

	return r
}

func (s *randomSource) Float64() float64 {
	s.scanner.Scan()
	f, err := strconv.ParseFloat(s.scanner.Text(), 64)
	if err != nil {
		panic(err)
	}

	return f
}

func (s *randomSource) Close() {
	s.f.Close()
}
