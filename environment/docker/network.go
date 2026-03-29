package docker

import (
	"context"
	"strconv"
	"strings"

	"emperror.dev/errors"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"

	"github.com/Minenetpro/pelican-wings/config"
)

func secureNetworkName(id string) string {
	return "pelican-" + id
}

func EnsureSecureNetwork(ctx context.Context, cli *client.Client, id string) (string, error) {
	name := secureNetworkName(id)

	if _, err := cli.NetworkInspect(ctx, name, network.InspectOptions{}); err == nil {
		return name, nil
	} else if !client.IsErrNotFound(err) {
		return "", errors.Wrap(err, "environment/docker: failed to inspect secure network")
	}

	enableIPv6 := false
	if _, err := cli.NetworkCreate(ctx, name, network.CreateOptions{
		Driver:     "bridge",
		EnableIPv6: &enableIPv6,
		Internal:   false,
		Attachable: false,
		Ingress:    false,
		ConfigOnly: false,
		Options: map[string]string{
			"com.docker.network.bridge.default_bridge":       "false",
			"com.docker.network.bridge.enable_icc":           "false",
			"com.docker.network.bridge.enable_ip_masquerade": "true",
			"com.docker.network.bridge.host_binding_ipv4":    "0.0.0.0",
			"com.docker.network.driver.mtu":                  strconv.FormatInt(config.Get().Docker.Network.NetworkMTU, 10),
		},
	}); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return name, nil
		}
		return "", errors.Wrap(err, "environment/docker: failed to create secure network")
	}

	return name, nil
}

func RemoveSecureNetwork(ctx context.Context, cli *client.Client, id string) error {
	if err := cli.NetworkRemove(ctx, secureNetworkName(id)); err != nil {
		if client.IsErrNotFound(err) {
			return nil
		}
		return errors.Wrap(err, "environment/docker: failed to remove secure network")
	}

	return nil
}
