#!/bin/bash
# Script to simplify all TestMain functions to use automatic cluster detection

# Find all main_test.go files
files=$(find test -name "main_test.go" -type f)

for file in $files; do
    echo "Simplifying $file..."

    # Create simplified content
    cat > "$file" << 'EOF'
package PACKAGE_NAME

import (
	"log"
	"os"
	"testing"

	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestMain(m *testing.M) {
	// No explicit cluster configuration needed!
	// The dynamic reconfiguration system will:
	// 1. Auto-detect current cluster state (or use defaults if fresh cluster)
	// 2. Reconfigure per-test based on test labels
	// 3. Tests with specific labels (LabelIstio, LabelExperimental, etc.) will
	//    automatically trigger cluster reconfiguration

	if err := suite.BeforeSuiteFunc(); err != nil {
		log.Printf("Setup failed: %v", err)
		os.Exit(1)
	}

	// Run tests
	exitCode := m.Run()

	// Cleanup after tests
	if err := suite.AfterSuiteFunc(); err != nil {
		log.Printf("Warning: cleanup failed: %v", err)
	}

	os.Exit(exitCode)
}
EOF

    # Extract package name from original file and update
    package_name=$(head -1 "$file" | awk '{print $2}')
    sed -i.bak "s/PACKAGE_NAME/$package_name/" "$file"
    rm "$file.bak"

    echo "  ✅ Simplified $file (package: $package_name)"
done

echo ""
echo "All TestMain functions simplified!"
echo "Tests will now use automatic cluster detection and reconfiguration."
