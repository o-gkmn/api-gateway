package router

import (
	"errors"
	"strings"
)

type pathPart struct {
	path string
	kind nodeType
}

func parsePath(p string) ([]pathPart, error) {
	parts := make([]pathPart, 0)

	ps := strings.Split(p, "/")
	buffer := ""
	for i, part := range ps {
		switch {
		case strings.HasPrefix(part, ":"):
			staticPart := pathPart{path: buffer + "/", kind: nodeStatic}
			paramPart := pathPart{path: part, kind: nodeParam}
			parts = append(parts, staticPart, paramPart)
			buffer = ""
		case strings.HasPrefix(part, "*"):
			if i != len(ps)-1 {
				return nil, errors.New("wildcard path must be placed at the end of the path")
			}
			staticPart := pathPart{path: buffer + "/", kind: nodeStatic}
			wildcardPart := pathPart{path: part, kind: nodeWildcard}
			parts = append(parts, staticPart, wildcardPart)
			buffer = ""
		default:
			if i == 0 {
				buffer += part
			} else {
				buffer += "/" + part
			}
		}
	}

	if buffer != "" {
		parts = append(parts, pathPart{path: buffer, kind: nodeStatic})
	}

	return parts, nil
}
