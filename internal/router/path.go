package router

import (
	"api-gateway/logger"
	"log/slog"
	"strings"
)

type pathPart struct {
	path string
	kind nodeType
}

func parsePath(p string) []pathPart {
	parts := make([]pathPart, 0)

	ps := strings.Split(p, "/")
	buffer := ""
	for i, part := range ps {
		switch {
		case strings.HasPrefix(part, ":"):
			paramPart := pathPart{
				path: part,
				kind: nodeParam,
			}
			staticPart := pathPart{
				path: buffer + "/",
				kind: nodeStatic,
			}
			parts = append(parts, staticPart)
			parts = append(parts, paramPart)
			buffer = ""
		case strings.HasPrefix(part, "*"):
			if i != len(ps)-1 {
				logger.Error("wildcard path must be placed end of the path", slog.String("part", part))
				panic("wildcard path must be placed end of the path")
			}
			wildcardPart := pathPart{
				path: part,
				kind: nodeWildcard,
			}
			staticPart := pathPart{
				path: buffer + "/",
				kind: nodeStatic,
			}
			parts = append(parts, staticPart)
			parts = append(parts, wildcardPart)
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
		staticPart := pathPart{
			path: buffer,
			kind: nodeStatic,
		}
		parts = append(parts, staticPart)
	}

	return parts
}
