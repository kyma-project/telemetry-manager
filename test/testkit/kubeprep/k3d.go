package kubeprep

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// importImageToK3D imports a local Docker image into the k3d cluster
func importImageToK3D(ctx context.Context, t TestingT, image, clusterName string) error {
	// First check if the image exists in the local Docker runtime
	checkCmd := exec.CommandContext(ctx, "docker", "image", "inspect", image)
	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("image %s not found in local Docker runtime", image)
	}

	cmd := exec.CommandContext(ctx, "k3d", "image", "import", image, "-c", clusterName)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to import image: %w\noutput: %s", err, output)
	}

	t.Logf("Imported image %s to k3d cluster %s", image, clusterName)

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
	if after, ok := strings.CutPrefix(contextName, "k3d-"); ok {
		return after, nil
	}

	return "", fmt.Errorf("not a k3d cluster context: %s", contextName)
}
