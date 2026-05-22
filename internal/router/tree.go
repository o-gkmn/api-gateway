package router

import (
	"api-gateway/logger"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
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

type paramEntry struct {
	key   string
	value string
}

type Params struct {
	entries [maxParams]paramEntry
	count   int
}

func (p *Params) snapshot() int { return p.count }
func (p *Params) restore(s int) { p.count = s }

func (p *Params) Get(key string) string {
	for i := 0; i < p.count; i++ {
		if p.entries[i].key == key {
			return p.entries[i].value
		}
	}
	return ""
}

type matchResult struct {
	handler        Handler
	params         *Params
	status         int
	allowedMethods []string
}

type Tree struct {
	root      *routeNode
	paramPool sync.Pool
}

func NewTree() *Tree {
	t := &Tree{
		root: &routeNode{prefix: "/", kind: nodeStatic},
	}
	t.paramPool.New = func() any { return &Params{} }
	return t
}

func (t *Tree) Insert(method methodIndex, path string, handler Handler) {
	p, err := parsePath(path)
	if err != nil {
		logger.Error("failed to parse path", slog.String("path", path), slog.String("error", err.Error()))
		panic(err.Error())
	}

	n := t.root
	for _, part := range p {
		n = insertPart(n, part)
	}

	if n.handlers[method] != nil {
		logger.Error("route handler is already defined", slog.String("path", path))
		panic(fmt.Sprintf("handler for path '%s' is already registered", path))
	}

	n.handlers[method] = handler

}

func (t *Tree) Match(method methodIndex, path string) matchResult {
	params := t.paramPool.Get().(*Params)
	params.count = 0
	result := tryMatch(t.root, path, params, method)
	if result.status == http.StatusOK {
		result.params = params
	} else {
		t.paramPool.Put(params)
	}
	return result
}

func (t *Tree) ReleaseParams(p *Params) {
	t.paramPool.Put(p)
}

func insertPart(n *routeNode, part pathPart) *routeNode {
	switch part.kind {
	case nodeParam, nodeWildcard:
		var err error
		n, err = insertDynamicNode(n, part.path, part.kind)
		if err != nil {
			logger.Error("failed to insert dynamic node", slog.String("error", err.Error()))
			panic(err.Error())
		}
		return n
	case nodeStatic:
		if n.kind == nodeStatic {
			return insertStaticNode(n, part.path)
		}

		for _, child := range n.staticChildren {
			if child.prefix[0] == part.path[0] {
				return insertStaticNode(child, part.path)
			}
		}
		newNode := &routeNode{prefix: part.path, kind: nodeStatic}
		n.staticChildren = append(n.staticChildren, newNode)
		return newNode
	}

	return n
}

func insertStaticNode(n *routeNode, remaining string) *routeNode {
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
				return insertStaticNode(child, rest)
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

func insertDynamicNode(n *routeNode, remaining string, kind nodeType) (*routeNode, error) {
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
			return nil, fmt.Errorf("%s node already has a different %s child", label, label)
		}
		return *slot, nil
	}

	child := &routeNode{
		prefix:    remaining,
		kind:      kind,
		paramName: paramName,
	}
	*slot = child
	return child, nil
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

func tryMatch(n *routeNode, remaining string, params *Params, method methodIndex) matchResult {
	switch n.kind {
	case nodeStatic:
		if len(remaining) < len(n.prefix) || remaining[:len(n.prefix)] != n.prefix {
			return matchResult{status: http.StatusNotFound}
		}
		remaining = remaining[len(n.prefix):]

	case nodeParam:
		slashIdx := strings.IndexByte(remaining, '/')
		if slashIdx == -1 {
			slashIdx = len(remaining)
		}

		paramValue := remaining[:slashIdx]

		if paramValue == "" {
			return matchResult{status: http.StatusNotFound}
		}

		params.entries[params.count] = paramEntry{key: n.paramName, value: paramValue}
		params.count++

		remaining = remaining[slashIdx:]

	case nodeWildcard:
		if remaining == "" {
			return matchResult{status: http.StatusNotFound}
		}
		params.entries[params.count] = paramEntry{key: n.paramName, value: remaining}
		params.count++
		remaining = ""

		if n.handlers[method] != nil {
			return matchResult{status: http.StatusOK, handler: n.handlers[method]}
		}
		allowedMethods := n.allowedMethodList()
		if len(allowedMethods) > 0 {
			return matchResult{status: http.StatusMethodNotAllowed, allowedMethods: allowedMethods}
		}
		return matchResult{status: http.StatusNotFound}
	}

	if remaining == "" {
		if n.handlers[method] != nil {
			return matchResult{
				status:  http.StatusOK,
				handler: n.handlers[method],
			}
		}

		allowedMethods := n.allowedMethodList()
		if len(allowedMethods) > 0 {
			return matchResult{
				status:         http.StatusMethodNotAllowed,
				allowedMethods: allowedMethods,
			}
		}

		return matchResult{
			status: http.StatusNotFound,
		}
	}

	snap := params.snapshot()
	var best405 matchResult
	best405.status = http.StatusNotFound

	tryChild := func(child *routeNode) matchResult {
		result := tryMatch(child, remaining, params, method)
		if result.status != http.StatusOK {
			params.restore(snap)
		}
		return result
	}

	for _, child := range n.staticChildren {
		if child.prefix[0] == remaining[0] {
			result := tryChild(child)
			if result.status == http.StatusOK {
				return result
			}
			if result.status == http.StatusMethodNotAllowed && best405.status != http.StatusMethodNotAllowed {
				best405 = result
			}
			break
		}
	}

	if n.paramChild != nil {
		result := tryChild(n.paramChild)
		if result.status == http.StatusOK {
			return result
		}
		if result.status == http.StatusMethodNotAllowed && best405.status != http.StatusMethodNotAllowed {
			best405 = result
		}
	}

	if n.wildChild != nil {
		result := tryChild(n.wildChild)
		if result.status == http.StatusOK {
			return result
		}
		if result.status == http.StatusMethodNotAllowed && best405.status != http.StatusMethodNotAllowed {
			best405 = result
		}
	}

	if best405.status == http.StatusMethodNotAllowed {
		return best405
	}

	return matchResult{
		status: http.StatusNotFound,
	}
}

func (n *routeNode) allowedMethodList() []string {
	methods := make([]string, 0, methodCount)
	for i, name := range methodNames {
		if n.handlers[i] != nil {
			methods = append(methods, name)
		}
	}
	return methods
}
