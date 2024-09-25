package gorm_generics

func ChunkSlice[T any](slice []T, chunkSize int) [][]T {
	var chunks [][]T
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize

		// necessary check to avoid slicing beyond
		// slice capacity
		if end > len(slice) {
			end = len(slice)
		}

		chunks = append(chunks, slice[i:end])
	}

	return chunks
}

func MapDto[M GormModel[T, M],T any](modelArray []M, dtoType T) []T {
	return Map(modelArray, func(ce M) T {
		et := ce.ToDto()
		etCasted, ok := any(et).(T)
		if !ok {
			return *new(T)
		}
		return etCasted
	})
}

// Maps a type to another using a function for transforming the data
func Map[T, U any](ts []T, f func(T) U) []U {
	us := make([]U, len(ts))
	for i := range ts {
		us[i] = f(ts[i])
	}
	return us
}
