package utils

import "strings"

func SplitN4(s string) [4]string {
	var result [4]string
	remain := s

	for i := 0; i < 3; i++ {
		idx := strings.IndexByte(remain, '.')
		if idx == -1 {
			result[i] = remain
			return result
		}
		result[i] = remain[:idx]
		remain = remain[idx+1:]
	}

	result[3] = remain
	return result
}
