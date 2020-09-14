package main

import (
	"C"
	"math"
)

type idxT C.ulonglong

type seriesOfVectors struct {
	series  ss_sample // sample type
	rows    idxT      // Length of feature vectors
	columns idxT      // Length of vector series
}

func NewSeriesOfVectors(rows idxT, columns idxT) *seriesOfVectors {
	return &seriesOfVectors{series: make(ss_sample, rows*columns), rows: rows, columns: columns}
}

func (s *seriesOfVectors) getCol(column idxT) ss_sample { return s.series[column*s.rows:] }

func vectorSumSquares(vec ss_sample, len int) float64 {
	sum := 0.0
	for i := 0; i < len; i++ {
		v1 := vec[i]
		sum += v1 * v1
	}
	return sum
}

func seriesSqrt(v []float64, seqlen, sz idxT) {
	seriesSum(v, seqlen, sz)
	for i := 0; i < int(sz-seqlen+1); i++ {
		v[i] = math.Sqrt(v[i])
	}
}

func seriesMean(v []float64, seqlen, sz idxT) {
	seriesSum(v, seqlen, sz)
	for i := 0; i < int(sz-seqlen+1); i++ {
		v[i] /= float64(seqlen)
	}
}

func seriesSum(v []float64, seqlen, sz idxT) {

	movingSum := 0.0
	for spd := 0; idxT(spd) < seqlen; spd++ {
		movingSum += v[spd]
	}

	for sp := 0; sp < int(sz-seqlen+1); sp++ {
		first := v[sp]
		last := v[idxT(sp)+seqlen]

		v[sp] = movingSum

		movingSum = movingSum - first + last
	}
}
