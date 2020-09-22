package main

import (
	"C"
	"math"
)

func vectorSumSquares(vec []float64, len int) float64 {
	sum := 0.0
	for i := 0; i < len; i++ {
		v1 := vec[i]
		sum += v1 * v1
	}
	return sum
}

func seriesSqrt(v []float64, seqlen, sz int) {
	seriesSum(v, seqlen, sz)
	for i := 0; i < int(sz-seqlen+1); i++ {
		v[i] = math.Sqrt(v[i])
	}
}

func seriesMean(v []float64, seqlen, sz int) {
	seriesSum(v, seqlen, sz)
	for i := 0; i < sz-seqlen+1; i++ {
		v[i] /= float64(seqlen)
	}
}

func seriesSum(v []float64, seqlen, sz int) {

	movingSum := 0.0
	for spd := 0; spd < seqlen; spd++ {
		movingSum += v[spd]
	}

	for sp := 0; sp < sz-seqlen+1; sp++ {
		first := v[sp]

		v[sp] = movingSum

		if sp+seqlen < len(v) {
			last := v[sp+seqlen]
			movingSum = movingSum - first + last
		}
	}
}
