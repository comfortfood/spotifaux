package spotifaux

import (
	"C"
	"math"
)

func VectorSumSquares(vec []float64, len int) float64 {
	sum := 0.0
	for i := 0; i < len; i++ {
		v1 := vec[i]
		sum += v1 * v1
	}
	return sum
}

func SeriesSqrt(v []float64, seqlen int) {
	SeriesSum(v, seqlen)
	for i := 0; i < len(v)-seqlen+1; i++ {
		v[i] = math.Sqrt(v[i])
	}
}

func SeriesMean(v []float64, seqlen int) {
	SeriesSum(v, seqlen)
	for i := 0; i < len(v)-seqlen+1; i++ {
		v[i] /= float64(seqlen)
	}
}

func SeriesSum(v []float64, seqlen int) {

	movingSum := 0.0
	for spd := 0; spd < seqlen; spd++ {
		movingSum += v[spd]
	}

	for sp := 0; sp < len(v)-seqlen; sp++ {
		first := v[sp]
		last := v[sp+seqlen]

		v[sp] = movingSum

		movingSum = movingSum - first + last
	}
	v[len(v)-seqlen] = movingSum
}
