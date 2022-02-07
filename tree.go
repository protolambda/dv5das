package main

import (
	"github.com/ethereum/go-ethereum/p2p/enode"
)

type Content interface {
	Depth() uint
	ID() enode.ID
	Score() float64
	SubtreeSize() uint
	// Add a node to the tree, return updated tree root, and ok == true if the node didn't already exist
	Add(n *enode.Node) (updated Content, ok bool)
	// Search for closest leaf nodes (log distance) and append to out,
	// maximum to the capacity of the out slice
	Search(target enode.ID, out []Content) []Content
	// Weakest finds the content with the weakest score at given tree depth
	Weakest(depth uint) Content
}

type LeafNode struct {
	depth uint
	score float64

	self *enode.Node
}

func bitCheck(id enode.ID, bitIndex uint) bool {
	return id[bitIndex>>3]&(1<<bitIndex) != 0
}

// zeroes out the remaining bits starting at depth index
func clip(id enode.ID, depth uint) enode.ID {
	i := depth >> 3
	id[i] &= (1 << (byte(depth) & 7)) - 1
	for j := i; j < uint(len(id)); j++ {
		id[j] = 0
	}
	return id
}

func (leaf *LeafNode) Depth() uint {
	return leaf.depth
}

func (leaf *LeafNode) ID() enode.ID {
	return leaf.self.ID()
}

func (leaf *LeafNode) Score() float64 {
	return leaf.score
}

func (*LeafNode) SubtreeSize() uint {
	return 1
}

func (leaf *LeafNode) Add(other *enode.Node) (updated Content, ok bool) {
	if leaf.ID() == other.ID() {
		return leaf, false
	}
	pair := &PairNode{depth: leaf.depth, score: 0, subtreeSize: 0, id: clip(leaf.ID(), leaf.depth)}
	_, _ = pair.Add(leaf.self)
	_, _ = pair.Add(other)
	return pair, true
}

func (leaf *LeafNode) Search(target enode.ID, out []Content) []Content {
	if len(out) == cap(out) {
		return out
	}
	return append(out, leaf)
}

func (leaf *LeafNode) Weakest(depth uint) Content {
	return leaf
}

type PairNode struct {
	depth       uint
	score       float64
	subtreeSize uint

	// Bits after depth index are zeroed
	id enode.ID

	// left and right are never nil at the same time

	// May be nil (pair node as extension node)
	left Content
	// May be nil (pair node as extension node)
	right Content
}

func (pair *PairNode) Depth() uint {
	return pair.depth
}

func (pair *PairNode) ID() enode.ID {
	return pair.id
}

func (pair *PairNode) Score() float64 {
	return pair.score
}

func (pair *PairNode) SubtreeSize() uint {
	return pair.subtreeSize
}

func (pair *PairNode) Add(n *enode.Node) (updated Content, ok bool) {
	if pair.ID() == n.ID() {
		return pair, false
	}
	if bitCheck(n.ID(), pair.depth) {
		if pair.right != nil {
			pair.right, ok = pair.right.Add(n)
		} else {
			pair.right = &LeafNode{
				depth: pair.depth + 1,
				score: 0,
				self:  n,
			}
			ok = true
		}
	} else {
		if pair.left != nil {
			pair.left, ok = pair.left.Add(n)
		} else {
			pair.left = &LeafNode{
				depth: pair.depth + 1,
				score: 0,
				self:  n,
			}
			ok = true
		}
	}
	if ok {
		pair.subtreeSize += 1
		pair.score = 0
		if pair.left != nil {
			pair.score += pair.left.Score()
		}
		if pair.right != nil {
			pair.score += pair.right.Score()
		}
	}
	return pair, ok
}

func (pair *PairNode) Search(target enode.ID, out []Content) []Content {
	if len(out) == cap(out) {
		return out
	}
	if pair.left == nil {
		return pair.right.Search(target, out)
	}
	if pair.right == nil {
		return pair.left.Search(target, out)
	}
	if bitCheck(target, pair.depth) {
		out = pair.right.Search(target, out)
		if len(out) < cap(out) {
			out = pair.left.Search(target, out)
		}
		return out
	} else {
		out = pair.left.Search(target, out)
		if len(out) < cap(out) {
			out = pair.right.Search(target, out)
		}
		return out
	}
}

func (pair *PairNode) Weakest(depth uint) Content {
	if depth > pair.depth {
		if pair.left == nil || (pair.right.Score() > pair.left.Score()) {
			return pair.right.Weakest(depth)
		}
		return pair.left.Weakest(depth)
	}
	return pair
}
