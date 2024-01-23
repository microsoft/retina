package loader

import (
	"context"
	"fmt"
	"os"
)

func WriteFile(ctx context.Context, destFile, text string) error {
	tmpFile := fmt.Sprintf("%s.tmp", destFile)
	outputFile, err := os.Create(tmpFile)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	_, err = outputFile.WriteString(text)
	if err != nil {
		return err
	}

	err = os.Rename(tmpFile, destFile)
	if err != nil {
		return err
	}
	return nil
}
