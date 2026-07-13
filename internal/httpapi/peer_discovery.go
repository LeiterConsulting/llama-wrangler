package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"
)

const (
	mdnsMulticastAddress = "224.0.0.251:5353"
	mdnsDiscoveryTimeout = 750 * time.Millisecond
)

var mdnsServiceNames = []string{
	"_llama-wrangler._tcp.local.",
	"_llama-wrangler-subscriber._tcp.local.",
	"_ollama._tcp.local.",
}

type PeerDiscoveryStatus struct {
	Mode       string                   `json:"mode"`
	Status     string                   `json:"status"`
	StartedAt  time.Time                `json:"started_at"`
	FinishedAt time.Time                `json:"finished_at"`
	Summary    map[string]int           `json:"summary"`
	MDNS       MDNSDiscoveryStatus      `json:"mdns"`
	SubnetScan SubnetScanStatus         `json:"subnet_scan"`
	Candidates []PeerDiscoveryCandidate `json:"candidates"`
	Warnings   []string                 `json:"warnings"`
	Adoption   PeerDiscoveryAdoption    `json:"adoption"`
}

type MDNSDiscoveryStatus struct {
	Enabled   bool     `json:"enabled"`
	Status    string   `json:"status"`
	Services  []string `json:"services"`
	TimeoutMS int      `json:"timeout_ms"`
	Message   string   `json:"message"`
}

type SubnetScanStatus struct {
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type PeerDiscoveryCandidate struct {
	ID               string   `json:"id"`
	DisplayName      string   `json:"display_name"`
	Service          string   `json:"service"`
	Instance         string   `json:"instance"`
	Host             string   `json:"host,omitempty"`
	Port             int      `json:"port,omitempty"`
	Addresses        []string `json:"addresses,omitempty"`
	EndpointURL      string   `json:"endpoint_url,omitempty"`
	ControlLevel     string   `json:"control_level"`
	TrustLevel       string   `json:"trust_level"`
	CapabilitySource string   `json:"capability_source"`
	ApprovalState    string   `json:"approval_state"`
	AdoptionPath     string   `json:"adoption_path"`
}

type PeerDiscoveryAdoption struct {
	ManagedNode     string `json:"managed_node"`
	PassiveEndpoint string `json:"passive_endpoint"`
}

type mdnsRecordSet struct {
	ptr  map[string][]string
	srv  map[string]mdnsSRV
	addr map[string][]string
}

type mdnsSRV struct {
	Target string
	Port   int
}

func (s *Server) discoverPeersStatus(ctx context.Context) PeerDiscoveryStatus {
	started := time.Now().UTC()
	status := PeerDiscoveryStatus{
		Mode:      "operator_initiated_mdns",
		Status:    "completed",
		StartedAt: started,
		Summary:   map[string]int{},
		MDNS: MDNSDiscoveryStatus{
			Enabled:   true,
			Status:    "running",
			Services:  append([]string{}, mdnsServiceNames...),
			TimeoutMS: int(mdnsDiscoveryTimeout / time.Millisecond),
			Message:   "One-shot mDNS/Bonjour discovery only. No subnet scan is performed.",
		},
		SubnetScan: SubnetScanStatus{
			Enabled: false,
			Status:  "disabled_requires_future_explicit_opt_in",
			Message: "Subnet scanning is disabled. Future subnet scans must require separate explicit operator opt-in.",
		},
		Warnings: []string{
			"Discovered peers are candidates only; they are not saved, approved, or routed automatically.",
			"Use Managed Node enrollment or Passive Endpoint add to adopt a candidate.",
		},
		Adoption: PeerDiscoveryAdoption{
			ManagedNode:     "Generate an enrollment token, install/enroll the Wrangler subscriber, then approve the Managed Node.",
			PassiveEndpoint: "Add an existing Ollama endpoint with an explicit trust level and safe /api/tags validation.",
		},
	}
	records, err := queryMDNS(ctx, mdnsServiceNames, mdnsDiscoveryTimeout)
	if err != nil {
		status.MDNS.Status = "unavailable"
		status.MDNS.Message = "mDNS discovery could not complete in this environment; manual enrollment remains available."
		status.Warnings = append(status.Warnings, "mDNS unavailable: "+safeDiscoveryError(err))
	} else {
		status.Candidates = mdnsCandidates(records)
		if len(status.Candidates) == 0 {
			status.MDNS.Status = "no_candidates_found"
			status.MDNS.Message = "mDNS probe completed with no Llama Wrangler or Ollama service candidates."
		} else {
			status.MDNS.Status = "candidates_found"
			status.MDNS.Message = "mDNS probe found candidate services. Review and adopt manually."
		}
	}
	status.FinishedAt = time.Now().UTC()
	status.Summary["candidates"] = len(status.Candidates)
	status.Summary["mdns_services_queried"] = len(status.MDNS.Services)
	if !status.SubnetScan.Enabled {
		status.Summary["subnet_scan_disabled"] = 1
	}
	return status
}

func queryMDNS(ctx context.Context, services []string, timeout time.Duration) (mdnsRecordSet, error) {
	records := mdnsRecordSet{ptr: map[string][]string{}, srv: map[string]mdnsSRV{}, addr: map[string][]string{}}
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return records, err
	}
	defer conn.Close()
	packet := buildMDNSQuery(services)
	dst, err := net.ResolveUDPAddr("udp4", mdnsMulticastAddress)
	if err != nil {
		return records, err
	}
	if _, err := conn.WriteTo(packet, dst); err != nil {
		return records, err
	}
	deadline := time.Now().Add(timeout)
	_ = conn.SetReadDeadline(deadline)
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return records, ctx.Err()
		default:
		}
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return records, err
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				return records, nil
			}
			return records, err
		}
		parsed, err := parseMDNSRecords(buf[:n])
		if err != nil {
			continue
		}
		mergeMDNSRecords(&records, parsed)
		if time.Now().After(deadline) {
			return records, nil
		}
	}
}

func buildMDNSQuery(services []string) []byte {
	packet := make([]byte, 12, 256)
	binary.BigEndian.PutUint16(packet[4:6], uint16(len(services)))
	for _, service := range services {
		packet = appendDNSName(packet, service)
		packet = binary.BigEndian.AppendUint16(packet, 12)
		packet = binary.BigEndian.AppendUint16(packet, 1)
	}
	return packet
}

func appendDNSName(packet []byte, name string) []byte {
	name = strings.TrimSuffix(name, ".")
	for _, label := range strings.Split(name, ".") {
		if label == "" {
			continue
		}
		if len(label) > 63 {
			label = label[:63]
		}
		packet = append(packet, byte(len(label)))
		packet = append(packet, label...)
	}
	return append(packet, 0)
}

func parseMDNSRecords(packet []byte) (mdnsRecordSet, error) {
	records := mdnsRecordSet{ptr: map[string][]string{}, srv: map[string]mdnsSRV{}, addr: map[string][]string{}}
	if len(packet) < 12 {
		return records, fmt.Errorf("short dns packet")
	}
	qd := int(binary.BigEndian.Uint16(packet[4:6]))
	an := int(binary.BigEndian.Uint16(packet[6:8]))
	ns := int(binary.BigEndian.Uint16(packet[8:10]))
	ar := int(binary.BigEndian.Uint16(packet[10:12]))
	offset := 12
	for i := 0; i < qd; i++ {
		var err error
		_, offset, err = readDNSName(packet, offset)
		if err != nil {
			return records, err
		}
		offset += 4
		if offset > len(packet) {
			return records, fmt.Errorf("short dns question")
		}
	}
	for i := 0; i < an+ns+ar; i++ {
		name, next, err := readDNSName(packet, offset)
		if err != nil {
			return records, err
		}
		if next+10 > len(packet) {
			return records, fmt.Errorf("short dns rr")
		}
		rrType := binary.BigEndian.Uint16(packet[next : next+2])
		rdLen := int(binary.BigEndian.Uint16(packet[next+8 : next+10]))
		rdata := next + 10
		offset = rdata + rdLen
		if offset > len(packet) {
			return records, fmt.Errorf("short dns rdata")
		}
		switch rrType {
		case 1:
			if rdLen == 4 {
				records.addr[normalizeDNSName(name)] = append(records.addr[normalizeDNSName(name)], net.IP(packet[rdata:offset]).String())
			}
		case 12:
			target, _, err := readDNSName(packet, rdata)
			if err == nil {
				service := normalizeDNSName(name)
				records.ptr[service] = append(records.ptr[service], normalizeDNSName(target))
			}
		case 28:
			if rdLen == 16 {
				records.addr[normalizeDNSName(name)] = append(records.addr[normalizeDNSName(name)], net.IP(packet[rdata:offset]).String())
			}
		case 33:
			if rdLen >= 6 {
				port := int(binary.BigEndian.Uint16(packet[rdata+4 : rdata+6]))
				target, _, err := readDNSName(packet, rdata+6)
				if err == nil {
					records.srv[normalizeDNSName(name)] = mdnsSRV{Target: normalizeDNSName(target), Port: port}
				}
			}
		}
	}
	return records, nil
}

func readDNSName(packet []byte, offset int) (string, int, error) {
	labels := []string{}
	original := offset
	jumped := false
	for depth := 0; depth < 32; depth++ {
		if offset >= len(packet) {
			return "", original, fmt.Errorf("dns name out of bounds")
		}
		l := int(packet[offset])
		if l == 0 {
			offset++
			if jumped {
				return strings.Join(labels, ".") + ".", original, nil
			}
			return strings.Join(labels, ".") + ".", offset, nil
		}
		if l&0xC0 == 0xC0 {
			if offset+1 >= len(packet) {
				return "", original, fmt.Errorf("dns pointer out of bounds")
			}
			ptr := ((l & 0x3F) << 8) | int(packet[offset+1])
			if !jumped {
				original = offset + 2
				jumped = true
			}
			offset = ptr
			continue
		}
		offset++
		if offset+l > len(packet) {
			return "", original, fmt.Errorf("dns label out of bounds")
		}
		labels = append(labels, string(packet[offset:offset+l]))
		offset += l
	}
	return "", original, fmt.Errorf("dns name compression loop")
}

func mergeMDNSRecords(dst *mdnsRecordSet, src mdnsRecordSet) {
	for service, instances := range src.ptr {
		dst.ptr[service] = append(dst.ptr[service], instances...)
	}
	for instance, srv := range src.srv {
		dst.srv[instance] = srv
	}
	for host, addrs := range src.addr {
		dst.addr[host] = append(dst.addr[host], addrs...)
	}
}

func mdnsCandidates(records mdnsRecordSet) []PeerDiscoveryCandidate {
	var candidates []PeerDiscoveryCandidate
	seen := map[string]bool{}
	services := make([]string, 0, len(records.ptr))
	for service := range records.ptr {
		services = append(services, service)
	}
	sort.Strings(services)
	for _, service := range services {
		instances := append([]string{}, records.ptr[service]...)
		sort.Strings(instances)
		for _, instance := range instances {
			key := service + "|" + instance
			if seen[key] {
				continue
			}
			seen[key] = true
			srv := records.srv[instance]
			addrs := dedupeStrings(records.addr[srv.Target])
			host := strings.TrimSuffix(srv.Target, ".")
			endpoint := ""
			if srv.Port > 0 {
				hostForURL := host
				if len(addrs) > 0 {
					hostForURL = addrs[0]
				}
				if hostForURL != "" {
					endpoint = fmt.Sprintf("http://%s:%d", hostForURL, srv.Port)
				}
			}
			candidates = append(candidates, PeerDiscoveryCandidate{
				ID:               discoveryCandidateID(service, instance),
				DisplayName:      strings.TrimSuffix(instance, "."),
				Service:          service,
				Instance:         instance,
				Host:             host,
				Port:             srv.Port,
				Addresses:        addrs,
				EndpointURL:      endpoint,
				ControlLevel:     "candidate",
				TrustLevel:       "lan_unverified",
				CapabilitySource: "mdns_observed",
				ApprovalState:    "not_added",
				AdoptionPath:     discoveryAdoptionPath(service),
			})
		}
	}
	return candidates
}

func discoveryCandidateID(service, instance string) string {
	sum := sha256.Sum256([]byte(service + "|" + instance))
	return "mdns_" + hex.EncodeToString(sum[:6])
}

func discoveryAdoptionPath(service string) string {
	if strings.Contains(service, "ollama") {
		return "passive_endpoint_add"
	}
	return "managed_node_enrollment"
}

func normalizeDNSName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" || strings.HasSuffix(name, ".") {
		return name
	}
	return name + "."
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func safeDiscoveryError(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	if len(text) > 160 {
		text = text[:160]
	}
	return strings.ReplaceAll(text, "\n", " ")
}
