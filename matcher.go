package spotifaux

import (
	"container/list"
	"math"
)

type matcher struct {
	frameQueue list.List
	matchedFilter
}

// Push a frame onto the frameQueue
// and pop the last frame from the queue
func (m *matcher) pushFrameQueue() {
	for m.frameQueue.Len() > 0 {
		e := m.frameQueue.Front()
		m.frameQueue.Remove(e)
	}
}

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
func (m *matcher) match(s *soundSpotter) int {
	dist := 0.0
	minD := 1e6
	dRadius := 0.0
	minDist := 10.0
	winner := -1
	// Perform the recursive Matched Filtering (core match algorithm)
	m.execute(s.ShingleSize, s.dbSize)
	qN0 := m.qNorm[0] // pre-calculate denominator coefficient
	// DD now contains (1 x N) multi-dimensional matched filter output
	for k := 0; k < s.dbSize-s.ShingleSize+1; k++ {
		sk := m.sNorm[k]
		pk := s.dbPowersCurrent[k]
		if !math.IsNaN(pk) && !(sk == NEGINF) && pk > s.pwr_abs_thresh {
			// The norm matched filter distance  is the Euclidean distance between the vectors
			dist = 2 - 2/(qN0*sk)*m.DD[k] // squared Euclidean distance
			dRadius = math.Abs(dist)      // Distance from search radius
			// Perform min-dist search
			if dRadius < minD { // prefer matches at front
				minD = dRadius
				minDist = dist
				winner = k
			}
		}
	}
	// FIX ME: the frame queue hash table logic is a bit off when queue sizes (or window sizes) change
	if winner > -1 {
		m.pushFrameQueue() // Hash down frame to hop boundary and queue
	} else if m.frameQueue.Len() > 0 {
		e := m.frameQueue.Front()
		m.frameQueue.Remove(e)
	}
	dist = minDist
	return winner
}
