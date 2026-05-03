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
