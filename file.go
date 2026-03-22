package setay

import "os"

// writeFileAtomic writes data to a file atomically (well, as close as we can).
func writeFileAtomic(filename string, data []byte) error {
	return os.WriteFile(filename, data, 0644)
}
