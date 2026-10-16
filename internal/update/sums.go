package update

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

func parseSums(data, name string) ([]byte, error) {
	var found []byte
	for line := range strings.Lines(data) {
		line = strings.TrimSuffix(line, "\n")
		if line == "" {
			continue
		}
		hexSum, file, ok := strings.Cut(line, "  ")
		sum, err := hex.DecodeString(hexSum)
		if !ok || err != nil || len(sum) != sha256.Size || file == "" {
			return nil, fmt.Errorf("%w: bad line in %s", ErrChecksum, SumsName)
		}
		if file != name {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("%w: %s listed twice", ErrChecksum, name)
		}
		found = sum
	}
	if found == nil {
		return nil, fmt.Errorf("%w: %s missing from %s", ErrChecksum, name, SumsName)
	}
	return found, nil
}
