package paseto

import "encoding/binary"

func PAE(pieces ...[]byte) []byte {
	pieceCount := make([]byte, 8)
	binary.LittleEndian.PutUint64(pieceCount, uint64(len(pieces)))

	totalBytes := len(pieceCount)

	for _, piece := range pieces {
		totalBytes += len(piece) + 8
	}

	result := make([]byte, 0, totalBytes)

	result = append(result, pieceCount...)

	for _, piece := range pieces {
		lenBuf := make([]byte, 8)
		binary.LittleEndian.PutUint64(lenBuf, uint64(len(piece)))
		result = append(result, lenBuf...)
		result = append(result, piece...)
	}

	return result
}
