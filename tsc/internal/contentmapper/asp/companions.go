package asp

import (
	"fmt"
	"os"
	"sort"
)

// Companion declarations belong only to programs containing their source file.
// They are ambient contracts, not automatic implementation annotations.
func (m *MappedFile) companionFiles() ([]string, error) {
	var files []string
	for source := range m.Sources {
		file := source + ".d.ts"
		info, err := os.Stat(file)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			return nil, fmt.Errorf("companion declaration is a directory: %s", file)
		}
		files = append(files, file)
	}
	sort.Strings(files)
	return files, nil
}
