package storage

import (
	"strings"
)

type radixNode struct {
	key      string
	children map[byte]*radixNode
	isEnd    bool
	value    interface{}
}

func newRadixNode(key string) *radixNode {
	return &radixNode{
		key:      key,
		children: make(map[byte]*radixNode),
		isEnd:    false,
	}
}

type RadixTree struct {
	root *radixNode
	size int
}

func NewRadixTree() *RadixTree {
	return &RadixTree{
		root: newRadixNode(""),
		size: 0,
	}
}

func (rt *RadixTree) Insert(key string, value interface{}) {
	if key == "" {
		return
	}

	rt.insert(rt.root, key, value)
}

func (rt *RadixTree) insert(node *radixNode, key string, value interface{}) {
	if key == "" {
		if !node.isEnd {
			rt.size++
		}
		node.isEnd = true
		node.value = value
		return
	}

	firstChar := key[0]
	child, exists := node.children[firstChar]

	if !exists {
		newNode := newRadixNode(key)
		newNode.isEnd = true
		newNode.value = value
		node.children[firstChar] = newNode
		rt.size++
		return
	}

	commonPrefix := rt.longestCommonPrefix(child.key, key)
	
	if commonPrefix == child.key {
		rt.insert(child, key[len(commonPrefix):], value)
		return
	}

	if commonPrefix == key {
		splitNode := newRadixNode(child.key[len(commonPrefix):])
		splitNode.children = child.children
		splitNode.isEnd = child.isEnd
		splitNode.value = child.value

		child.key = commonPrefix
		child.children = make(map[byte]*radixNode)
		child.children[splitNode.key[0]] = splitNode
		child.isEnd = true
		child.value = value
		rt.size++
		return
	}

	parentNode := newRadixNode(commonPrefix)
	
	existingNode := newRadixNode(child.key[len(commonPrefix):])
	existingNode.children = child.children
	existingNode.isEnd = child.isEnd
	existingNode.value = child.value

	newNode := newRadixNode(key[len(commonPrefix):])
	newNode.isEnd = true
	newNode.value = value

	parentNode.children[existingNode.key[0]] = existingNode
	parentNode.children[newNode.key[0]] = newNode

	child.key = parentNode.key
	child.children = parentNode.children
	child.isEnd = parentNode.isEnd
	child.value = parentNode.value

	rt.size++
}

func (rt *RadixTree) Search(key string) (interface{}, bool) {
	if key == "" {
		return nil, false
	}

	node := rt.search(rt.root, key)
	if node != nil && node.isEnd {
		return node.value, true
	}
	return nil, false
}

func (rt *RadixTree) search(node *radixNode, key string) *radixNode {
	if key == "" {
		return node
	}

	firstChar := key[0]
	child, exists := node.children[firstChar]
	if !exists {
		return nil
	}

	if strings.HasPrefix(key, child.key) {
		return rt.search(child, key[len(child.key):])
	}

	return nil
}

func (rt *RadixTree) HasPrefix(prefix string) bool {
	if prefix == "" {
		return true
	}

	return rt.hasPrefix(rt.root, prefix)
}

func (rt *RadixTree) hasPrefix(node *radixNode, prefix string) bool {
	if prefix == "" {
		return true
	}

	firstChar := prefix[0]
	child, exists := node.children[firstChar]
	if !exists {
		return false
	}

	if len(prefix) <= len(child.key) {
		return strings.HasPrefix(child.key, prefix)
	}

	if strings.HasPrefix(prefix, child.key) {
		return rt.hasPrefix(child, prefix[len(child.key):])
	}

	return false
}

func (rt *RadixTree) MatchesWildcard(domain string) (interface{}, bool) {
	parts := strings.Split(domain, ".")
	
	for i := 1; i < len(parts); i++ {
		suffix := strings.Join(parts[i:], ".")
		if value, exists := rt.Search(suffix); exists {
			return value, true
		}
	}
	
	return nil, false
}

func (rt *RadixTree) Size() int {
	return rt.size
}

func (rt *RadixTree) longestCommonPrefix(str1, str2 string) string {
	minLen := len(str1)
	if len(str2) < minLen {
		minLen = len(str2)
	}

	i := 0
	for i < minLen && str1[i] == str2[i] {
		i++
	}

	return str1[:i]
}

func (rt *RadixTree) Clear() {
	rt.root = newRadixNode("")
	rt.size = 0
}