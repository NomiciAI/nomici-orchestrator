package sandbox

import "os/exec"

func Detect(mode string) Availability {
	mode = NormalizeMode(mode)
	switch mode {
	case ModeNone:
		return Availability{
			Provider: ProviderDisabled,
			Mode:     ModeNone,
			Status:   StatusDisabled,
			Message:  "sandbox disabled",
		}
	case ModeContainer:
		if binary, ok := FirstAvailableBinary("docker", "podman", "container"); ok {
			return Availability{
				Provider:      ProviderContainerRuntime,
				Mode:          ModeContainer,
				Status:        StatusAvailable,
				RuntimeBinary: binary,
				Message:       "container runtime available",
			}
		}
		return Availability{
			Provider: ProviderContainerRuntime,
			Mode:     ModeContainer,
			Status:   StatusUnavailable,
			Message:  "container sandbox configured but docker, podman, or container was not found",
		}
	default:
		return Availability{
			Provider: ProviderLocalWorkspace,
			Mode:     ModeLocal,
			Status:   StatusAvailable,
			Message:  "local workspace sandbox available",
		}
	}
}

func FirstAvailableBinary(names ...string) (string, bool) {
	for _, name := range names {
		if _, err := exec.LookPath(name); err == nil {
			return name, true
		}
	}
	return "", false
}
