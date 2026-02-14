package kubeprep

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// importImageToK3D imports a local Docker image into the k3d cluster
func importImageToK3D(ctx context.Context, t TestingT, image, clusterName string) error {
	t.Helper()

	t.Logf("Importing local image %s into k3d cluster %s...", image, clusterName)

	// First check if the image exists in the local Docker runtime
	checkCmd := exec.CommandContext(ctx, "docker", "image", "inspect", image)
	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("image %s not found in local Docker runtime - please build it first (e.g., make docker-build IMG=%s)", image, image)
	}

	cmd := exec.CommandContext(ctx, "k3d", "image", "import", image, "-c", clusterName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to import image: %w\noutput: %s", err, output)
	}

	t.Log("Successfully imported image to k3d")
	return nil
}

// detectK3DCluster returns the name of the current k3d cluster
func detectK3DCluster(ctx context.Context) (string, error) {
	// Get current kubectl context
	cmd := exec.CommandContext(ctx, "kubectl", "config", "current-context")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current context: %w", err)
	}

	contextName := strings.TrimSpace(string(output))

	// k3d clusters typically have context like "k3d-<cluster-name>"
	if strings.HasPrefix(contextName, "k3d-") {
		return strings.TrimPrefix(contextName, "k3d-"), nil
	}

	return "", fmt.Errorf("not a k3d cluster context: %s", contextName)
}
