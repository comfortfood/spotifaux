package main

import (
	"math"
)

var NEGINF = math.Inf(-1)

type matchedFilter struct {
	D     [][]float64 // cross-correlation matrix
	DD    []float64   // matched filter result vector
	qNorm []float64   // query L2 norm vector
	sNorm ss_sample   // database L2 norm vector

	maxShingleSize int // largest shingle to allocate
	maxDBSize      int // largest database to allocate
}

func (f *matchedFilter) resize(maxShingleSize, maxDBSize int) {
	if f.maxShingleSize != 0 {
		f.clearMemory()
	}

	f.maxShingleSize = maxShingleSize
	f.maxDBSize = maxDBSize

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
func (f *matchedFilter) clearMemory() {
	if f.D != nil {
		f.D = nil
	}

	if f.DD != nil {
		f.DD = nil
	}

	if f.qNorm != nil {
		f.qNorm = nil
	}

	if f.sNorm != nil {
		f.sNorm = nil
	}
}

func (f *matchedFilter) getQNorm(i int) float64 {
	if i < f.maxShingleSize {
		return f.qNorm[i]
	} else {
		return 0
	}
}

func (f *matchedFilter) getSNorm(i int) float64 {
	if i < f.maxDBSize {
		return f.sNorm[i]
	} else {
		return 0
	}
}

func (f *matchedFilter) getDD(i int) float64 {
	if i < f.maxDBSize {
		return f.DD[i]
	} else {
		return 0
	}
}

// Incremental multidimensional time series insert
func (f *matchedFilter) insert(s *soundSpotter, muxi int) {

	// incrementally compute cross correlation matrix
	f.incrementalCrossCorrelation(s, muxi)

	// Keep input L2 norms for correct shingle norming at distance computation stage
	qPtr := s.loFeature
	if s.inShingle.getCol(idxT(muxi))[qPtr] > NEGINF {
		f.qNorm[muxi] = vectorSumSquares(s.inShingle.getCol(idxT(muxi))[qPtr:], s.hiFeature-s.loFeature+1)
	} else {
		f.qNorm[muxi] = 0.0
	}
}

func (f *matchedFilter) incrementalCrossCorrelation(s *soundSpotter, muxi int) {

	ioff := (idxT(muxi) * s.inShingle.rows) + idxT(s.loFeature)
	doff := (idxT(muxi+s.LoK) * s.inShingle.rows) + idxT(s.loFeature)
	isp := s.inShingle.series[ioff:]
	dsp := s.dbShingles
	totalLen := s.dbSize - s.shingleSize - s.HiK - s.LoK + 1
	dim := s.hiFeature - s.loFeature + 1
	dpp := muxi
	// Make Correlation matrix entry for this frame against entire source database
	for ; totalLen > 0; totalLen-- {
		qp := 0    // input column pointer
		sp := doff // db column pointer
		doff += s.inShingle.rows
		// point to correlation cell j,k
		f.D[muxi][dpp] = 0.0 // initialize correlation cell
		l := dim             // Size of bounded feature vector
		for ; l > 0; l-- {
			f.D[muxi][dpp] += isp[qp] * dsp[sp]
			qp++
			sp++
		}
		dpp++
	}
}

func (f *matchedFilter) sumCrossCorrMatrixDiagonals(shingleSize, dbSize, loK, hiK int) {
	dp := 0

	// Matched Filter length W hop H over N frames
	for k := loK; k < dbSize-hiK-shingleSize+1; k++ {
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
	for k := s.LoK; k < s.getLengthSourceShingles()-s.HiK; k++ {
		sPtr := s.dbShingles[k*s.cqtN:]
		if sPtr[s.loFeature] > NEGINF {
			f.sNorm[k] = vectorSumSquares(sPtr[s.loFeature:], s.hiFeature-s.loFeature+1)
		} else {
			f.sNorm[k] = 0.0
		}
	}
	seriesSqrt(f.sNorm, idxT(s.shingleSize), idxT(s.getLengthSourceShingles()))
}

// PRE-CONDITIONS:
// a complete input shingle (W feature frames)
// D (W x N) and DD (1 x N) are already allocated by Matcher constructor
// D (W x N) is a normed cross-correlation matrix between input features and sources
func (f *matchedFilter) execute(shingleSize, dbSize, loK, hiK int) {
	// sum diagonals of cross-correlation matrix
	f.sumCrossCorrMatrixDiagonals(shingleSize, dbSize, loK, hiK)
	// Perform query shingle norming
	seriesSqrt(f.qNorm, idxT(shingleSize), idxT(shingleSize))
}
