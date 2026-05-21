package router

import (
	"api-gateway/logger"
	"fmt"
)

type nodeType uint8

const (
	nodeStatic nodeType = iota
	nodeParam
	nodeWildcard
)

type routeNode struct {
	prefix         string
	kind           nodeType
	paramName      string
	staticChildren []*routeNode
	paramChild     *routeNode
	wildChild      *routeNode
	handlers       [methodCount]Handler
}

func (r *Router) insertInto(n *routeNode, remaining string) *routeNode {
	i := commonPrefixLength(n.prefix, remaining)

	// CASE 1: The prefix has not been fully consumed; a split is required
	if i < len(n.prefix) {
		splitNode(n, i)

		rest := remaining[i:]
		if len(rest) > 0 {
			newNode := &routeNode{
				prefix: rest,
				kind:   nodeStatic,
			}

			n.staticChildren = append(n.staticChildren, newNode)
			return newNode
		}

		return n
	}

	// CASE 2: The n.prefix has been fully consumed, but the remaining has not been fully consumed
	if i == len(n.prefix) && i < len(remaining) {
		rest := remaining[i:]
		for _, child := range n.staticChildren {
			if rest[0] == child.prefix[0] {
				return r.insertInto(child, rest)
			}
		}

		newNode := &routeNode{
			prefix: rest,
			kind:   nodeStatic,
		}

		n.staticChildren = append(n.staticChildren, newNode)
		return newNode
	}

	// CASE 3: n.prefix and remaining have been fully consumed
	if i == len(remaining) && i == len(n.prefix) {
		return n
	}

	panic("unreachable: insertInto must hit one of the three cases")
}

func insertDynamicNode(n *routeNode, remaining string, kind nodeType) *routeNode {
	paramName := remaining[1:]

	var slot **routeNode
	var label string
	if kind == nodeParam {
		slot = &n.paramChild
		label = "param"
	} else {
		slot = &n.wildChild
		label = "wild"
	}

	if *slot != nil {
		if (*slot).paramName != paramName {
			logger.Error(label + " child is already in " + label + " node")
			panic(label + " child is already in " + label + " node")
		}
		return *slot
	}

	child := &routeNode{
		prefix:    remaining,
		kind:      kind,
		paramName: paramName,
	}
	*slot = child
	return child
}

func splitNode(n *routeNode, i int) {
	child := &routeNode{
		prefix:         n.prefix[i:],
		kind:           n.kind,
		staticChildren: n.staticChildren,
		paramName:      n.paramName,
		paramChild:     n.paramChild,
		wildChild:      n.wildChild,
		handlers:       n.handlers,
	}

	n.prefix = n.prefix[:i]
	n.kind = nodeStatic
	n.paramName = ""
	n.paramChild = nil
	n.wildChild = nil
	n.handlers = [methodCount]Handler{}
	n.staticChildren = []*routeNode{child}
}

func commonPrefixLength(a, b string) int {
	minLen := len(a)
	if minLen > len(b) {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return minLen
}

func printTree(n *routeNode, prefix string, isLast bool, isRoot bool) {
	if isRoot {
		fmt.Printf("%q\n", n.prefix)
	} else {
		connector := "├── "
		if isLast {
			connector = "└── "
		}
		fmt.Printf("%s%s%q\n", prefix, connector, n.prefix)
	}

	childPrefix := prefix
	if !isRoot {
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}
	}

	for i, c := range n.staticChildren {
		printTree(c, childPrefix, i == len(n.staticChildren)-1, false)
	}
}

func (r *Router) findHandler(method, path string) (Handler, error) {
	return nil, fmt.Errorf("route not found: %s %s", method, path)
}
