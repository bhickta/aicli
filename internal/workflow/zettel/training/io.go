package training

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

func readJSONLLines(path string, visit func([]byte) error) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open training archive %s: %w", path, err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadBytes('\n')
		if len(strings.TrimSpace(string(line))) > 0 {
			if visitErr := visit(line); visitErr != nil {
				return visitErr
			}
		}
		if err == nil {
			continue
		}
		if err == io.EOF {
			return nil
		}
		return fmt.Errorf("read training archive %s: %w", path, err)
	}
}
