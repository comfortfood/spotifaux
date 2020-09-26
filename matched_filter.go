package spotifaux

import (
	"math"
)

var NEGINF = math.Inf(-1)

type matchedFilter struct {
	D  [][]float64 // cross-correlation matrix
	DD []float64   // matched filter result vector
}

func (f *matchedFilter) resize(maxShingleSize, maxDBSize int) {
	// Cross-correlation matrix
	f.D = make([][]float64, maxShingleSize)
	for i := 0; i < maxShingleSize; i++ {
		f.D[i] = make([]float64, maxDBSize)
	}

	// Matched filter matrix
	f.DD = make([]float64, maxDBSize)
}

// Incremental multidimensional time series insert
func (f *matchedFilter) Insert(s *soundSpotter, muxi int) {

	// incrementally compute cross correlation matrix
	f.incrementalCrossCorrelation(s, muxi)
}

func (f *matchedFilter) incrementalCrossCorrelation(s *soundSpotter, muxi int) {
	// Make Correlation matrix entry for this frame against entire source database
	for dpp := 0; dpp < s.LengthSourceShingles; dpp++ {
		coor := 0.0 // initialize correlation cell
		for _, qp := range s.ChosenFeatures {
			coor += s.InShingles[muxi][qp] * s.dbShingles[muxi+dpp][qp]
		}
		f.D[muxi][muxi+dpp] = coor
	}
}

func (f *matchedFilter) sumCrossCorrMatrixDiagonals(s *soundSpotter) {
	// Matched Filter length W hop H over N frames
	for k := 0; k < s.LengthSourceShingles; k++ {
		f.DD[k] = 0.0
		for l := 0; l < s.ShingleSize; l++ {
			f.DD[k] += f.D[l][k+l] // Sum rest of k's diagonal up to W elements
		}
	}
}

// PRE-CONDITIONS:
// a complete input shingle (W feature frames)
// D (W x N) and DD (1 x N) are already allocated by Matcher constructor
// D (W x N) is a normed cross-correlation matrix between input features and sources
func (f *matchedFilter) execute(s *soundSpotter) {
	// sum diagonals of cross-correlation matrix
	f.sumCrossCorrMatrixDiagonals(s)
}
