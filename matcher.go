package main

type matcher struct {
	maxShingleSize int
	maxDBSize      int
	frameQueue     []int
	frameHashTable []int
}

func (m *matcher) clearFrameQueue() {
	for i := 0; i < len(m.frameQueue); i++ {
		m.frameQueue[i] = 0
	}
	for i := 0; i < len(m.frameHashTable); i++ {
		m.frameHashTable[i] = 0
	}
}
