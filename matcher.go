package spotifaux

import (
	"math"
)

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
func Match(datFileName string, s *SoundSpotter) (int, float64, error) {

	dr, err := NewDatReader(datFileName, s.CqtN)
	if err != nil {
		return 0, 0.0, err
	}

	dist := 0.0
	minD := 1e6
	dRadius := 0.0
	minDist := 10.0
	winner := -1

	qN0 := 0.0 // TODO: more efficient with a SeriesSum
	for muxi := 0; muxi < s.ShingleSize; muxi++ {
		for _, qp := range s.ChosenFeatures {
			qN0 += s.InShingles[muxi][qp] * s.InShingles[muxi][qp]
		}
	}
	qN0 = math.Sqrt(qN0)

	front := 0
	dbShingles := make([][]float64, s.ShingleSize)
	for dpp := 0; dpp < s.ShingleSize; dpp++ {
		dbShingles[dpp], err = dr.Dat()
		if err != nil {
			return 0, 0.0, err
		}
	}

	// Make Correlation matrix entry for this frame against entire source database
	for dpp := 0; dpp < dr.frames; dpp++ {

		sk := 0.0 // TODO: more efficient with a SeriesSum
		DD := 0.0
		for muxi := 0; muxi < s.ShingleSize; muxi++ {
			m := (front + muxi) % s.ShingleSize
			if dbShingles[m] == nil {
				break
			}
			for _, qp := range s.ChosenFeatures {
				sk += dbShingles[m][qp] * dbShingles[m][qp]
				DD += s.InShingles[muxi][qp] * dbShingles[m][qp]
			}
		}
		sk = math.Sqrt(sk)

		// The norm matched filter distance  is the Euclidean distance between the vectors
		dist = 2 - 2/(qN0*sk)*DD // squared Euclidean distance
		dRadius = math.Abs(dist) // Distance from search radius
		// Perform min-dist search
		if dRadius < minD { // prefer matches at front
			minD = dRadius
			minDist = dist
			winner = dpp
		}

		if dpp+s.ShingleSize < dr.frames {
			dbShingles[front], err = dr.Dat()
			if err != nil {
				return 0, 0.0, err
			}
		} else {
			dbShingles[front] = nil
		}
		front = (front + 1) % s.ShingleSize
	}
	return winner, minDist, nil
}
