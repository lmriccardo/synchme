package utils

// ChunkData divides the input data into chunks of size at most size
func ChunkData(data []byte, size uint32) [][]byte {
	// We do not want the size of each chunk to be
	// less than 1 actually.
	if size == 0 {
		return nil
	}

	var chunks [][]byte          // Define the result of the function
	input_data_size := len(data) // Get the size of the input data

	for data_idx := 0; data_idx < input_data_size; data_idx += int(size) {
		// Compute the end index of the current chunk and create it
		end_data_idx := min(data_idx+int(size), input_data_size)
		chunk := make([]byte, end_data_idx-data_idx)
		copy(chunk, data[data_idx:end_data_idx])

		// Append the current chunk into the result
		chunks = append(chunks, chunk)
	}

	return chunks
}
