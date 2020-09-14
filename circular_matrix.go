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

func (s *seriesOfVectors) insert(outputFeatures ss_sample, column idxT) {
	copy(s.series[column*s.rows:], outputFeatures[:s.rows])
}

func (s *seriesOfVectors) copy(source *seriesOfVectors) int {
	if s.columns != source.columns || s.rows != source.rows {
		return 0
	}
	copy(s.series, source.series)
	return 1
}

func vectorSumSquares(vec ss_sample, len int) float64 {
	sum1 := 0.0
	for i := 0; i < len; i++ {
		v1 := vec[i]
		sum1 += v1 * v1
	}
	return sum1
}

func seriesSqrt(v []float64, seqlen, sz idxT) {
	seriesSum(v, seqlen, sz)
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

func seriesSum(v []float64, seqlen, sz idxT) {

	movingSum := 0.0
	for spd := 0; idxT(spd) < seqlen; spd++ {
		movingSum += v[spd]
	}

	for sp := 0; sp <= int(sz-seqlen); sp++ {
		first := v[sp]
		last := v[idxT(sp)+seqlen]

		v[sp] = movingSum

		movingSum = movingSum - first + last
	}
}
