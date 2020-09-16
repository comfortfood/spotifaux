package main

import (
	"bufio"
	"os"
	"strconv"
)

type source interface {
	Float64() float64
	Close() error
}

type fixedSource struct {
	f       *os.File
	scanner *bufio.Scanner
}

func newFixedSource() *fixedSource {
	r := &fixedSource{}

	f, err := os.Open("/Users/wyatttall/git/BLAST/soundspotter/out")
	if err != nil {
		panic(err)
	}
	r.f = f

	r.scanner = bufio.NewScanner(f)
	r.scanner.Split(bufio.ScanLines)

	return r
}

func (s *fixedSource) Float64() float64 {
	s.scanner.Scan()
	v, err := strconv.ParseFloat(s.scanner.Text(), 64)
	if err != nil {
		panic(err)
	}

	return v
}

func (s *fixedSource) Close() error {
	return s.f.Close()
}
