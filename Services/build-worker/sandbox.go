package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// contestantUser is the unprivileged user the contestant process runs as.
// 65534:65534 is the conventional nobody:nogroup pair. Combined with CapDrop
// ALL, no-new-privileges, and a read-only rootfs this prevents the contestant
// from acting as root inside the sandbox.
const contestantUser = "65534:65534"

// buildSecurityOpt assembles the Docker SecurityOpt list and fails closed. When
// a profile is configured it must be usable; when the sandbox is strict (set in
// non-dev environments) both seccomp and AppArmor are mandatory. Returning an
// error here aborts the launch and fails the build rather than silently running
// the contestant unconfined.
func buildSecurityOpt(cfg Config) ([]string, error) {
	opts := []string{"no-new-privileges:true"}

	// seccomp. Docker SecurityOpt requires the JSON content inline, not a path.
	if cfg.SeccompProfile != "" {
		data, err := os.ReadFile(cfg.SeccompProfile)
		if err != nil {
			return nil, fmt.Errorf("seccomp profile %q unreadable: %w", cfg.SeccompProfile, err)
		}
		if len(bytes.TrimSpace(data)) == 0 {
			return nil, fmt.Errorf("seccomp profile %q is empty", cfg.SeccompProfile)
		}
		opts = append(opts, "seccomp="+string(data))
	} else if cfg.StrictSandbox {
		return nil, errors.New("strict sandbox: SECCOMP_PROFILE is required but not set")
	}

	// AppArmor. The profile is referenced by name and must be loaded on the host
	// kernel beforehand; Docker rejects an unknown profile at container start.
	if cfg.AppArmorProfile != "" {
		opts = append(opts, "apparmor="+cfg.AppArmorProfile)
	} else if cfg.StrictSandbox {
		return nil, errors.New("strict sandbox: APPARMOR_PROFILE is required but not set")
	}

	return opts, nil
}

// launchSandbox creates and starts a hardened contestant container and returns
// its container ID, its IP on the contestant network, and the internal port
// (8080). Build-worker and bot-fleet reach the contestant by that IP because
// they are attached to the same network.
func launchSandbox(ctx context.Context, cli *client.Client, cfg Config, imageName, submissionID string) (containerID, ip string, port int, err error) {
	exposed := nat.PortSet{"8080/tcp": struct{}{}}

	pidsLimit := int64(50)
	mem := cfg.SandboxMemoryMB * 1024 * 1024

	containerCfg := &container.Config{
		Image:        imageName,
		ExposedPorts: exposed,
		User:         contestantUser,
		Labels: map[string]string{
			"trade-eval":    "contestant",
			"submission-id": submissionID,
		},
	}

	// Sandbox confinement profiles fail closed (see buildSecurityOpt): a
	// configured-but-unreadable profile, or a missing required profile in strict
	// mode, aborts the launch rather than running the contestant unconfined.
	secOpt, err := buildSecurityOpt(cfg)
	if err != nil {
		return "", "", 0, err
	}

	hostCfg := &container.HostConfig{
		Resources: container.Resources{
			Memory:     mem,
			MemorySwap: mem,
			CPUQuota:   100000,
			CpusetCpus: cfg.SandboxCPUCores,
			PidsLimit:  &pidsLimit,
		},
		NetworkMode:    container.NetworkMode(cfg.ContestantNetwork),
		ReadonlyRootfs: true,
		Tmpfs:          map[string]string{"/tmp": "rw,noexec,nosuid,size=64m"},
		SecurityOpt:    secOpt,
		CapDrop:        []string{"ALL"},
		CapAdd:         []string{},
		AutoRemove:     false,
		RestartPolicy:  container.RestartPolicy{Name: "no"},
	}

	netCfg := &network.NetworkingConfig{}

	created, err := cli.ContainerCreate(ctx, containerCfg, hostCfg, netCfg, nil, "contestant-"+submissionID)
	if err != nil {
		return "", "", 0, fmt.Errorf("container create: %w", err)
	}
	containerID = created.ID

	if err := cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return containerID, "", 0, fmt.Errorf("container start: %w", err)
	}

	inspect, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return containerID, "", 0, fmt.Errorf("container inspect: %w", err)
	}

	// Resolve the container IP on the contestant network so callers on that same
	// network reach it on port 8080.
	if inspect.NetworkSettings != nil {
		if netInfo, ok := inspect.NetworkSettings.Networks[cfg.ContestantNetwork]; ok {
			ip = netInfo.IPAddress
		}
		if ip == "" {
			ip = inspect.NetworkSettings.IPAddress
		}
	}

	port = 8080 // contestant containers always listen on 8080

	return containerID, ip, port, nil
}
