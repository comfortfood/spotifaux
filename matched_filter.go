package main

import "math"

var NEGINF = math.Inf(-1)

type matchedFilter struct {
	D     [][]float64 // cross-correlation matrix
	DD    []float64   // matched filter result vector
	qNorm []float64   // query L2 norm vector
	sNorm ss_sample   // database L2 norm vector

	maxShingleSize int // largest shingle to allocate
	maxDBSize      int // largest database to allocate
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
func (f *matchedFilter) insert(inShingle *seriesOfVectors, shingleSize int, dbShingles *seriesOfVectors, dbSize, muxi,
	loFeature, hiFeature, loK, hiK int) {

	// incrementally compute cross correlation matrix
	f.incrementalCrossCorrelation(inShingle, shingleSize, dbShingles, dbSize, muxi, loFeature, hiFeature, loK, hiK)

	// Keep input L2 norms for correct shingle norming at distance computation stage
	qPtr := loFeature
	if inShingle.getCol(idxT(muxi))[qPtr] > NEGINF {
		f.qNorm[muxi] = vectorSumSquares(inShingle.getCol(idxT(muxi))[qPtr:], hiFeature-loFeature+1)
	} else {
		f.qNorm[muxi] = 0.0
	}
}

func (f *matchedFilter) incrementalCrossCorrelation(inShingle *seriesOfVectors, shingleSize int, dbShingles *seriesOfVectors,
	dbSize, muxi, loFeature, hiFeature, loK, hiK int) {
	var sp, qp idxT
	var l int
	numFeatures := inShingle.getRows()
	ioff := (idxT(muxi) * numFeatures) + idxT(loFeature)
	doff := (idxT(muxi+loK) * numFeatures) + idxT(loFeature)
	isp := inShingle.getSeries()[ioff:]
	dsp := dbShingles.getSeries()
	totalLen := dbSize - shingleSize - hiK - loK + 1
	dim := hiFeature - loFeature + 1
	dpp := muxi
	// Make Correlation matrix entry for this frame against entire source database
	for ; totalLen > 0; totalLen-- {
		qp = 0    // input column pointer
		sp = doff // db column pointer
		doff += numFeatures
		// point to correlation cell j,k
		f.D[muxi][dpp] = 0.0 // initialize correlation cell
		l = dim              // Size of bounded feature vector
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
	var k, l, w int

	// Matched Filter length W hop H over N frames
	for k = loK; k < dbSize-hiK-shingleSize+1; k++ {
		// initialize matched filter output k
		f.DD[k] = f.D[0][k] // DD+k <- q_0 . s_k
		l = shingleSize - 1
		dp = 1
		w = k + 1 // next diagonal element
		for ; l > 0; l-- {
			f.DD[k] += f.D[dp][w] // Sum rest of k's diagonal up to W elements
			dp++
			w++
		}
	}
}

func (f *matchedFilter) updateDatabaseNorms(dbShingles *seriesOfVectors, shingleSize, dbSize,
	loFeature, hiFeature, loK, hiK int) {
	var sPtr ss_sample
	for k := loK; k < dbSize-hiK; k++ {
		sPtr = dbShingles.getCol(idxT(k))
		if sPtr[loFeature] > NEGINF {
			f.sNorm[k] = vectorSumSquares(sPtr[loFeature:], hiFeature-loFeature+1)
		} else {
			f.sNorm[k] = 0.0
		}
	}
	seriesSqrt(f.sNorm, idxT(shingleSize), idxT(dbSize))
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
