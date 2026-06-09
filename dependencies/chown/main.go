package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	argCount   = 3
	ownerParts = 2
)

func main() {
	if len(os.Args) != argCount {
		fmt.Fprintf(os.Stderr, "usage: chown <uid:gid> <path>\n")
		os.Exit(1)
	}

	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run(ownerArg, targetPath string) error {
	parts := strings.SplitN(ownerArg, ":", ownerParts)
	if len(parts) != ownerParts {
		return fmt.Errorf("invalid owner format %q, expected uid:gid", ownerArg)
	}

	uid, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("invalid uid %q: %w", parts[0], err)
	}

	gid, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid gid %q: %w", parts[1], err)
	}

	//nolint:gosec // G703/G122: targetPath is a trusted mount path passed by the operator, not user input; Lchown on symlinks is intentional (we own the volume).
	return filepath.Walk(targetPath, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		return os.Lchown(path, uid, gid)
	})
}
