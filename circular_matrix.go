package spotifaux

import (
	"C"
	"math"
)

func VectorSumSquares(vec []float64, chosenFeatures []int) float64 {
	sum := 0.0
	for _, i := range chosenFeatures {
		v1 := vec[i]
		sum += v1 * v1
	}
	return sum
}

func SeriesSqrt(v []float64, seqlen int) {
	SeriesSum(v, seqlen)
	for i := 0; i < len(v); i++ {
		v[i] = math.Sqrt(v[i])
	}
}

func SeriesMean(v []float64, seqlen int) {
	SeriesSum(v, seqlen)
	for i := 0; i < len(v); i++ {
		v[i] /= float64(seqlen)
	}
}

func SeriesSum(v []float64, seqlen int) {

	movingSum := 0.0
	for spd := 0; spd < seqlen; spd++ {
		movingSum += v[spd]
	}

	for sp := 0; sp < len(v); sp++ {
		first := v[sp]
		last := 0.0
		if sp+seqlen < len(v) {
			last = v[sp+seqlen]
		}

		v[sp] = movingSum

		movingSum = movingSum - first + last
	}
}
