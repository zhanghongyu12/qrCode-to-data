package payload

// BlockCount 返回 data 按 blockSize 分块的块数。
// blockSize<=0 或 data 为空时返回 0。
func BlockCount(data []byte, blockSize int) int {
	if blockSize <= 0 || len(data) == 0 {
		return 0
	}
	return (len(data) + blockSize - 1) / blockSize
}

// Chunk 将 data 按 blockSize 分块，返回各块字节切片（指向原数据，不拷贝）。
// 尾部不足一块时保留实际长度。
func Chunk(data []byte, blockSize int) [][]byte {
	n := BlockCount(data, blockSize)
	if n == 0 {
		return nil
	}
	chunks := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		start := i * blockSize
		end := start + blockSize
		if end > len(data) {
			end = len(data)
		}
		chunks = append(chunks, data[start:end])
	}
	return chunks
}
