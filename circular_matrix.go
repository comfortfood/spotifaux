package main

import (
	"C"
	"math"
)

type idxT C.ulonglong

type seriesOfVectors struct {
	series ss_sample // sample type
	M      idxT      // Length of feature vectors
	N      idxT      // Length of vector series
}

func NewSeriesOfVectors(M idxT, N idxT) *seriesOfVectors {
	return &seriesOfVectors{series: make(ss_sample, M*N), M: M, N: N}
}

// Column 0 Accessor
func (s *seriesOfVectors) getSeries() ss_sample { return s.series }

// Column accessor
func (s *seriesOfVectors) getCol(n idxT) ss_sample { return s.series[n*s.M:] }

// Dimension accessor
func (s *seriesOfVectors) getRows() idxT {
	return s.M
}

func (s *seriesOfVectors) getCols() idxT {
	return N
}

func (s *seriesOfVectors) insert(v ss_sample, j idxT) {
	qp := j * s.M
	fp := 0
	l := s.M
	for ; l > 0; l-- {
		s.series[qp] = v[fp] // copy feature extract output to inShingle current pos (muxi)
		qp++
		fp++
	}
}

func (s *seriesOfVectors) copy(source *seriesOfVectors) int {
	if !(s.getCols() == source.getCols() && s.getRows() == source.getRows()) {
		// NEED AN ERROR MESSAGE
		return 0
	}
	copy(s.series, source.getSeries())
	return 1
}

func vectorSumSquares(vec ss_sample, len int) float64 {
	sum1 := 0.0
	var v1 float64
	i := 0
	for ; len > 0; len-- {
		v1 = vec[i]
		sum1 += v1 * v1
		i++
	}
	return sum1
}

func seriesSqrt(v []float64, seqlen, sz idxT) {
	seriesSum(v, seqlen, sz) // <<<<<****** INTERCEPT NANS or INFS here ?
	l := sz - seqlen + 1
	i := 0
	for ; l > 0; l-- {
		v[i] = math.Sqrt(v[i])
		i++
	}
}

func seriesMean(v []float64, seqlen, sz idxT) {
	seriesSum(v, seqlen, sz)
	oneOverSeqLen := 1.0 / float64(seqlen)
	l := sz - seqlen + 1
	i := 0
	for ; l > 0; l-- {
		v[i] *= oneOverSeqLen
		i++
	}
}

//<<<<<****** INTERCEPT NANS or INFS here ?
func seriesSum(v []float64, seqlen, sz idxT) {
	tmp1 := 0.0
	tmp2 := 0.0
	sp := 0
	spd := 1
	l := seqlen - 1
	tmp1 = v[sp]
	// Initialize with first value
	for ; l > 0; l-- {
		v[sp] += v[spd]
		spd++
	}
	// Now walk the array subtracting first and adding last
	l = sz - seqlen // +1 -1
	sp = 1
	for ; l > 0; l-- {
		tmp2 = v[sp]
		v[sp] = v[sp-1] - tmp1 + v[idxT(sp)+seqlen-1]
		tmp1 = tmp2
		sp++
	}
}
