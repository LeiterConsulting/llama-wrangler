package httpapi

import (
	"net"
	"strings"
)

type networkExposure struct {
	Enabled bool   `json:"enabled"`
	Warning string `json:"warning,omitempty"`
}

func lanExposureForListen(listen string) networkExposure {
	host := listenHost(listen)
	if host == "" {
		return networkExposure{
			Enabled: true,
			Warning: "Llama Wrangler is listening on all interfaces. Keep this behind trusted networks and require admin/client tokens before using LAN access.",
		}
	}
	if strings.EqualFold(host, "localhost") {
		return networkExposure{}
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return networkExposure{
			Enabled: true,
			Warning: "Llama Wrangler is listening on a non-local hostname. Confirm this host only resolves on trusted networks and keep auth enabled.",
		}
	}
	if ip.IsLoopback() {
		return networkExposure{}
	}
	return networkExposure{
		Enabled: true,
		Warning: "Llama Wrangler is reachable beyond localhost. Use this only on trusted LANs with admin and client API-key auth enabled.",
	}
}

func listenHost(listen string) string {
	listen = strings.TrimSpace(listen)
	host, _, err := net.SplitHostPort(listen)
	if err == nil {
		return strings.Trim(host, "[]")
	}
	if strings.HasPrefix(listen, ":") {
		return ""
	}
	if strings.Count(listen, ":") == 1 {
		return strings.TrimSpace(strings.SplitN(listen, ":", 2)[0])
	}
	return strings.Trim(listen, "[]")
}
