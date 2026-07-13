package httpapi

import (
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMDNSCandidatesFromRecordsAreMetadataOnly(t *testing.T) {
	packet := make([]byte, 12)
	service := "_ollama._tcp.local."
	instance := "Studio Ollama._ollama._tcp.local."
	host := "studio.local."
	packet = appendDNSRR(packet, service, 12, appendDNSName(nil, instance))
	packet = appendDNSRR(packet, instance, 33, appendSRVRData(11434, host))
	packet = appendDNSRR(packet, host, 1, []byte{192, 0, 2, 10})
	binary.BigEndian.PutUint16(packet[6:8], 3)

	records, err := parseMDNSRecords(packet)
	if err != nil {
		t.Fatalf("parse records: %v", err)
	}
	candidates := mdnsCandidates(records)
	if len(candidates) != 1 {
		t.Fatalf("candidates = %#v", candidates)
	}
	candidate := candidates[0]
	if candidate.EndpointURL != "http://192.0.2.10:11434" {
		t.Fatalf("endpoint URL = %q", candidate.EndpointURL)
	}
	if candidate.TrustLevel != "lan_unverified" || candidate.ApprovalState != "not_added" || candidate.AdoptionPath != "passive_endpoint_add" {
		t.Fatalf("candidate metadata = %#v", candidate)
	}
	rendered, _ := json.Marshal(candidate)
	for _, forbidden := range []string{"SECRET_PROMPT", "Authorization", "lw_admin_", "lw_client_", "lw_hb_", "lw_enroll_"} {
		if strings.Contains(string(rendered), forbidden) {
			t.Fatalf("candidate leaked forbidden marker %q: %s", forbidden, string(rendered))
		}
	}
}

func TestDiscoverPeersEndpointIsOptInMetadataOnlyAndDoesNotPersistNodes(t *testing.T) {
	server := newIsolatedTestServer(t)
	initialNodes := len(server.store.Snapshot().Nodes)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/setup/discover-peers", nil)
	server.discoverPeers(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("discover peers status = %d body = %s", rr.Code, rr.Body.String())
	}
	var body PeerDiscoveryStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode discovery body: %v", err)
	}
	if body.Mode != "operator_initiated_mdns" || !body.MDNS.Enabled {
		t.Fatalf("discovery mode/mdns = %#v", body)
	}
	if body.SubnetScan.Enabled || body.SubnetScan.Status != "disabled_requires_future_explicit_opt_in" {
		t.Fatalf("subnet scan was not safely disabled: %#v", body.SubnetScan)
	}
	if len(server.store.Snapshot().Nodes) != initialNodes {
		t.Fatalf("discovery persisted nodes: before=%d after=%d", initialNodes, len(server.store.Snapshot().Nodes))
	}
	for _, candidate := range body.Candidates {
		if candidate.ApprovalState != "not_added" || candidate.TrustLevel != "lan_unverified" {
			t.Fatalf("unsafe candidate metadata = %#v", candidate)
		}
	}
	for _, forbidden := range []string{"SECRET_PROMPT", "Authorization", "lw_admin_", "lw_client_", "lw_hb_", "lw_enroll_", "token_hash"} {
		if strings.Contains(rr.Body.String(), forbidden) {
			t.Fatalf("discovery response leaked forbidden marker %q: %s", forbidden, rr.Body.String())
		}
	}
}

func appendDNSRR(packet []byte, name string, rrType uint16, rdata []byte) []byte {
	packet = appendDNSName(packet, name)
	packet = binary.BigEndian.AppendUint16(packet, rrType)
	packet = binary.BigEndian.AppendUint16(packet, 1)
	packet = binary.BigEndian.AppendUint32(packet, 120)
	packet = binary.BigEndian.AppendUint16(packet, uint16(len(rdata)))
	return append(packet, rdata...)
}

func appendSRVRData(port int, target string) []byte {
	var rdata []byte
	rdata = binary.BigEndian.AppendUint16(rdata, 0)
	rdata = binary.BigEndian.AppendUint16(rdata, 0)
	rdata = binary.BigEndian.AppendUint16(rdata, uint16(port))
	return appendDNSName(rdata, target)
}
