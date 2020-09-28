package spotifaux

import (
	"math"
)

var NEGINF = math.Inf(-1)

// Matching algorithm using recursive matched filter algorithm
// This algorithm is based on factoring the multi-dimensional convolution
// between the current input shingle and the database shingles
//
// The sum-of-products re-factoring reduces the number of multiplications
// required by an order of magnitude or more.
//
// Author: Michael A. Casey, April 24th - November 12th 2006
// Substantially Modified: Michael A. Casey, August 24th - 27th 2007
// Factored out dependency on SoundSpotter class, August 8th - 9th 2009
// Added power features for threshold tests
func Match(s *soundSpotter, qN0 float64, sNorm []float64) int {

	dist := 0.0
	minD := 1e6
	dRadius := 0.0
	minDist := 10.0
	winner := -1

	G := make([][]float64, s.ShingleSize)
	for dpp := 0; dpp < s.ShingleSize-1; dpp++ {
		G[dpp] = s.dbShingles[dpp]
	}
	g := s.ShingleSize - 1

	// Make Correlation matrix entry for this frame against entire source database
	for dpp := 0; dpp < s.LengthSourceShingles; dpp++ {

		if dpp+s.ShingleSize-1 < s.LengthSourceShingles {
			G[g] = s.dbShingles[dpp+s.ShingleSize-1]
		} else {
			G[g] = nil
		}
		g = (g + 1) % s.ShingleSize

		DD := 0.0
		for muxi := 0; muxi < s.ShingleSize; muxi++ {
			y := (g + muxi) % s.ShingleSize
			if G[y] == nil {
				break
			}
			for _, qp := range s.ChosenFeatures {
				DD += s.InShingles[muxi][qp] * G[y][qp]
			}
		}

		sk := sNorm[dpp]
		if sk != NEGINF {
			// The norm matched filter distance  is the Euclidean distance between the vectors
			dist = 2 - 2/(qN0*sk)*DD // squared Euclidean distance
			dRadius = math.Abs(dist) // Distance from search radius
			// Perform min-dist search
			if dRadius < minD { // prefer matches at front
				minD = dRadius
				minDist = dist
				winner = dpp
			}
		}
	}
	dist = minDist
	return winner
}
