package main

import (
	"container/list"
	"math"
)

type matcher struct {
	maxShingleSize int
	maxDBSize      int
	frameQueue     list.List
	frameHashTable []int
	matchedFilter
	useRelativeThreshold bool
}

// Push a frame onto the frameQueue
// and pop the last frame from the queue
func (m *matcher) pushFrameQueue() {
	for m.frameQueue.Len() > 0 {
		e := m.frameQueue.Front()
		m.frameHashTable[e.Value.(int)] = 0
		m.frameQueue.Remove(e)
	}
}

func (m *matcher) clearFrameQueue() {
	m.frameQueue.Init()
	p := 0
	mx := m.maxDBSize
	for ; mx > 0; mx-- {
		m.frameHashTable[p] = 0
		p++
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
func (m *matcher) match(shingleSize, dbSize, loDataLoc, hiDataLoc int,
	inPwMn float64, powers ss_sample, pwr_abs_thresh float64) int {
	pwr_rel_thresh := 0.1
	dist := 0.0
	minD := 1e6
	dRadius := 0.0
	minDist := 10.0
	winner := -1
	// Perform the recursive Matched Filtering (core match algorithm)
	m.execute(shingleSize, dbSize, loDataLoc, hiDataLoc)
	qN0 := m.getQNorm(0) // pre-calculate denominator coefficient
	// DD now contains (1 x N) multi-dimensional matched filter output
	oneOverW := 1.0 / float64(shingleSize)
	for k := loDataLoc; k < dbSize-shingleSize-hiDataLoc+1; k++ {
		// Test frame Queue
		if m.frameHashTable[int(float64(k)*oneOverW)] == 0 {
			sk := m.getSNorm(k)
			pk := powers[k]
			if !math.IsNaN(pk) && !(sk == NEGINF) && pk > pwr_abs_thresh &&
				(!m.useRelativeThreshold || inPwMn/pk < pwr_rel_thresh) {
				// The norm matched filter distance  is the Euclidean distance between the vectors
				dist = 2 - 2/(qN0*sk)*m.getDD(k) // squared Euclidean distance
				dRadius = math.Abs(dist)         // Distance from search radius
				// Perform min-dist search
				if dRadius < minD { // prefer matches at front
					minD = dRadius
					minDist = dist
					winner = k
				}
			}
		}
	}
	// New size is smaller
	// Reset frames beyond queueSize
	sz := m.frameQueue.Len()
	e := m.frameQueue.Back()
	for k := 0; k < sz; k++ {
		kVal := e.Value.(int)
		e = e.Prev()
		m.frameHashTable[kVal] = 0
	}
	// FIX ME: the frame queue hash table logic is a bit off when queue sizes (or window sizes) change
	if winner > -1 {
		m.pushFrameQueue() // Hash down frame to hop boundary and queue
	} else if m.frameQueue.Len() > 0 {
		e := m.frameQueue.Front()
		m.frameHashTable[e.Value.(int)] = 0
		m.frameQueue.Remove(e)
	}
	dist = minDist
	return winner
}
