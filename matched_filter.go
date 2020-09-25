package spotifaux

import (
	"math"
)

var NEGINF = math.Inf(-1)

type matchedFilter struct {
	D     [][]float64 // cross-correlation matrix
	DD    []float64   // matched filter result vector
	qNorm []float64   // query L2 norm vector
	sNorm []float64   // database L2 norm vector
}

func (f *matchedFilter) resize(maxShingleSize, maxDBSize int) {
	// Cross-correlation matrix
	f.D = make([][]float64, maxShingleSize)
	for i := 0; i < maxShingleSize; i++ {
		f.D[i] = make([]float64, maxDBSize)
	}

	// Matched filter matrix
	f.DD = make([]float64, maxDBSize)

	// Allocate for L2 norm vectors
	f.qNorm = make([]float64, maxShingleSize) // query is one shingle of length W
	f.sNorm = make([]float64, maxDBSize)      // source shingles of length W
}

// Incremental multidimensional time series insert
func (f *matchedFilter) Insert(s *soundSpotter, muxi int) {

	// incrementally compute cross correlation matrix
	f.incrementalCrossCorrelation(s, muxi)

	// Keep input L2 norms for correct shingle norming at distance computation stage
	qPtr := s.loFeature
	if s.InShingles[muxi][qPtr] > NEGINF {
		f.qNorm[muxi] = VectorSumSquares(s.InShingles[muxi][qPtr:], s.hiFeature-s.loFeature+1)
	} else {
		f.qNorm[muxi] = 0.0
	}
}

func (f *matchedFilter) incrementalCrossCorrelation(s *soundSpotter, muxi int) {
	// Make Correlation matrix entry for this frame against entire source database
	for dpp := 0; dpp < s.dbSize-s.ShingleSize+1; dpp++ {
		coor := 0.0 // initialize correlation cell
		for qp := 0; qp < s.hiFeature-s.loFeature+1; qp++ {
			coor += s.InShingles[muxi][s.loFeature+qp] * s.dbShingles[muxi+dpp][s.loFeature+qp]
		}
		f.D[muxi][muxi+dpp] = coor
	}
}

func (f *matchedFilter) sumCrossCorrMatrixDiagonals(shingleSize, dbSize int) {
	dp := 0

	// Matched Filter length W hop H over N frames
	for k := 0; k < dbSize-shingleSize+1; k++ {
		// initialize matched filter output k
		f.DD[k] = f.D[0][k] // DD+k <- q_0 . s_k
		l := shingleSize - 1
		dp = 1
		w := k + 1 // next diagonal element
		for ; l > 0; l-- {
			f.DD[k] += f.D[dp][w] // Sum rest of k's diagonal up to W elements
			dp++
			w++
		}
	}
}

func (f *matchedFilter) updateDatabaseNorms(s *soundSpotter) {
	for k := 0; k < s.LengthSourceShingles; k++ {
		sPtr := s.dbShingles[k]
		if sPtr[s.loFeature] > NEGINF {
			f.sNorm[k] = VectorSumSquares(sPtr[s.loFeature:], s.hiFeature-s.loFeature+1)
		} else {
			f.sNorm[k] = 0.0
		}
	}
	SeriesSqrt(f.sNorm, s.ShingleSize)
}

// PRE-CONDITIONS:
// a complete input shingle (W feature frames)
// D (W x N) and DD (1 x N) are already allocated by Matcher constructor
// D (W x N) is a normed cross-correlation matrix between input features and sources
func (f *matchedFilter) execute(shingleSize, dbSize int) {
	// sum diagonals of cross-correlation matrix
	f.sumCrossCorrMatrixDiagonals(shingleSize, dbSize)
	// Perform query shingle norming
	SeriesSqrt(f.qNorm, shingleSize)
}
