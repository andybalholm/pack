package brotli

// A Hasher maintains a hash table for finding backreferences in data.
type Hasher interface {
	// Init allocates the Hasher's internal storage, or clears it if
	// it is already allocated. Init must be called before any of the other
	// methods.
	Init()

	// Store puts an entry in the hash table for the data at index.
	Store(data []byte, index int)

	// Candidates hashes the data at index, fetches a list of possible matches
	// from the hash table, and appends the list to dst.
	Candidates(dst []int, data []byte, index int) []int
}
