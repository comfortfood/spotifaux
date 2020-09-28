package spotifaux

import (
	"C"
)

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
