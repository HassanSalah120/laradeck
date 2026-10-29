package storage

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// ReplaceFileAtomic atomically replaces targetPath with tmpPath on Windows using MoveFileEx.
// It never deletes targetPath prior to replacement.
func ReplaceFileAtomic(tmpPath, targetPath string) error {
	from, err := windows.UTF16PtrFromString(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	to, err := windows.UTF16PtrFromString(targetPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	err = windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("atomic file replacement failed: %w", err)
	}
	return nil
}
