package repository_after

import (
	"container/heap"
)

// Node represents a node in the Huffman tree.
type Node struct {
	value int // -1 for internal, 0-255 for leaves
	freq  uint64
	left  *Node
	right *Node
	// index for heap
	index int
}

// Code represents a Huffman code as its bit value and length.
type Code struct {
	Val uint64
	Len int
}

// priority queue implementation for Node
type nodeHeap []*Node

func (h nodeHeap) Len() int { return len(h) }
func (h nodeHeap) Less(i, j int) bool {
	if h[i].freq != h[j].freq {
		return h[i].freq < h[j].freq
	}
	return h[i].value < h[j].value
}
func (h nodeHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *nodeHeap) Push(x interface{}) {
	n := x.(*Node)
	n.index = len(*h)
	*h = append(*h, n)
}
func (h *nodeHeap) Pop() interface{} {
	old := *h
	n := old[len(old)-1]
	n.index = -1
	*h = old[:len(old)-1]
	return n
}

// buildTree builds a Huffman tree from frequencies.
func buildTree(freqs map[byte]uint64) *Node {
	var h nodeHeap
	// deterministic insertion order: sorted by byte value
	keys := make([]int, 0, len(freqs))
	for b := range freqs {
		keys = append(keys, int(b))
	}
	// sort keys
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	for _, kb := range keys {
		b := byte(kb)
		n := &Node{value: int(b), freq: freqs[b]}
		h = append(h, n)
	}
	if len(h) == 0 {
		return nil
	}
	heap.Init(&h)
	for h.Len() > 1 {
		a := heap.Pop(&h).(*Node)
		b := heap.Pop(&h).(*Node)
		merged := &Node{value: -1, freq: a.freq + b.freq, left: a, right: b}
		heap.Push(&h, merged)
	}
	return heap.Pop(&h).(*Node)
}

// generateCodes returns map[byte]Code (value, length).
func generateCodes(root *Node) map[byte]Code {
	codes := make(map[byte]Code)
	if root == nil {
		return codes
	}
	var walk func(n *Node, val uint64, length int)
	walk = func(n *Node, val uint64, length int) {
		if n == nil {
			return
		}
		if n.left == nil && n.right == nil && n.value >= 0 {
			// leaf
			if length == 0 {
				// single node tree, use '0'
				codes[byte(n.value)] = Code{Val: 0, Len: 1}
			} else {
				codes[byte(n.value)] = Code{Val: val, Len: length}
			}
			return
		}
		// left adds 0
		walk(n.left, val<<1, length+1)
		// right adds 1
		walk(n.right, (val<<1)|1, length+1)
	}
	walk(root, 0, 0)
	return codes
}
