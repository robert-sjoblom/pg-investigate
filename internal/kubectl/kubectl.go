package kubectl

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type VMIStatus struct {
	Status struct {
		NodeName string `json:"nodeName"`
	} `json:"status"`
}

func GetVMINode(context, vm, namespace string) (string, error) {
	cmd := exec.Command("kubectl", "--context", context, "get", "vmi", vm, "-n", namespace, "-o", "json")
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("kubectl failed: %s", string(exitErr.Stderr))
		}
		return "", err
	}

	var vmi VMIStatus
	if err := json.Unmarshal(out, &vmi); err != nil {
		return "", fmt.Errorf("failed to parse VMI JSON: %w", err)
	}

	return strings.TrimSpace(vmi.Status.NodeName), nil
}

type VMRef struct {
	Name      string
	Namespace string
	Context   string
	DC        string
}

// FindClusterVMs scans all given contexts for VMs whose name starts with cluster prefix
// in the given namespace. dcByContext maps the kubectl context to its DC short name.
func FindClusterVMs(cluster, namespace string, dcByContext map[string]string) ([]VMRef, error) {
	var found []VMRef
	for ctx, dc := range dcByContext {
		cmd := exec.Command("kubectl", "--context", ctx, "-n", namespace, "get", "vm", "-o", "name")
		out, err := cmd.Output()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return nil, fmt.Errorf("kubectl failed for %s: %s", ctx, string(exitErr.Stderr))
			}
			return nil, fmt.Errorf("kubectl failed for %s: %w", ctx, err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			name := strings.TrimPrefix(strings.TrimSpace(line), "virtualmachine.kubevirt.io/")
			if name == "" || !strings.HasPrefix(name, cluster+"-") {
				continue
			}
			found = append(found, VMRef{Name: name, Namespace: namespace, Context: ctx, DC: dc})
		}
	}
	return found, nil
}

func Run(command string) ([]byte, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("command failed: %w", err)
	}

	return out, nil
}
