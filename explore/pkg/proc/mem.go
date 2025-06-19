package proc

// MemoryReader is like io.ReaderAt, but the offset is a uint64 so that it
// can address all of 64-bit memory.
// Redundant with memoryReadWriter but more easily suited to working with
// the standard io package.
type MemoryReader interface {
	// ReadMemory is just like io.ReaderAt.ReadAt.
	ReadMemory(buf []byte, addr uint64) (n int, err error)
}

// MemoryReadWriter is an interface for reading or writing to
// the targets memory. This allows us to read from the actual
// target memory or possibly a cache.
type MemoryReadWriter interface {
	MemoryReader
	WriteMemory(addr uint64, data []byte) (written int, err error)
}
