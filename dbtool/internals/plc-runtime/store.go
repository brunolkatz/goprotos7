package plc_runtime

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/get-notify/gos7"
)

type ValueSnapshot struct {
	Address   string    `json:"address"`
	Value     string    `json:"value"`
	Error     string    `json:"error,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RuntimeStatus struct {
	Connected       bool            `json:"connected"`
	WSState         string          `json:"ws_state"`
	PLCState        string          `json:"plc_state"`
	Target          string          `json:"target,omitempty"`
	Rack            int             `json:"rack"`
	Slot            int             `json:"slot"`
	PollMS          int             `json:"poll_ms"`
	LastError       string          `json:"last_error,omitempty"`
	ConnectedSince  *time.Time      `json:"connected_since,omitempty"`
	LastReadAt      *time.Time      `json:"last_read_at,omitempty"`
	LastWSMessageAt *time.Time      `json:"last_ws_message_at,omitempty"`
	CPUInfo         *gos7.S7CpuInfo `json:"cpu_info,omitempty"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

var (
	valuesMu sync.RWMutex
	values   = map[string]ValueSnapshot{}
	status   = RuntimeStatus{
		Connected: false,
		WSState:   "DISCONNECTED",
		PLCState:  "DISCONNECTED",
	}

	dashboardDB   int32
	dashboardVars []string

	clientMu  sync.Mutex
	plcClient gos7.Client
)

func SetConnectionConfig(target string, rack, slot, pollMS int) {
	valuesMu.Lock()
	defer valuesMu.Unlock()
	status.Target = strings.TrimSpace(target)
	status.Rack = rack
	status.Slot = slot
	status.PollMS = pollMS
	status.UpdatedAt = time.Now()
}

func SetWSState(wsState string) {
	valuesMu.Lock()
	defer valuesMu.Unlock()
	normalized := strings.ToUpper(strings.TrimSpace(wsState))
	if normalized == "" {
		normalized = "DISCONNECTED"
	}
	status.WSState = normalized
	now := time.Now()
	status.LastWSMessageAt = &now
	status.UpdatedAt = now
}

func MarkWSMessage() {
	valuesMu.Lock()
	defer valuesMu.Unlock()
	now := time.Now()
	status.LastWSMessageAt = &now
	status.UpdatedAt = now
}

func SetConnecting() {
	valuesMu.Lock()
	defer valuesMu.Unlock()
	status.Connected = false
	status.PLCState = "CONNECTING"
	status.LastError = ""
	status.UpdatedAt = time.Now()
}

func SetConnected(target string) {
	valuesMu.Lock()
	defer valuesMu.Unlock()
	status.Connected = true
	status.Target = strings.TrimSpace(target)
	if status.PLCState == "" || status.PLCState == "DISCONNECTED" || status.PLCState == "CONNECTING" {
		status.PLCState = "CONNECTED"
	}
	status.LastError = ""
	now := time.Now()
	status.ConnectedSince = &now
	status.UpdatedAt = now
}

func SetDisconnected(errText string) {
	valuesMu.Lock()
	defer valuesMu.Unlock()
	status.Connected = false
	status.PLCState = "DISCONNECTED"
	status.LastError = strings.TrimSpace(errText)
	status.UpdatedAt = time.Now()
}

func SetPLCStatus(plcState string) {
	valuesMu.Lock()
	defer valuesMu.Unlock()
	normalized := strings.ToUpper(strings.TrimSpace(plcState))
	if normalized == "" || normalized == "UNKNOWN" {
		normalized = "DISCONNECTED"
	}
	status.PLCState = normalized
	status.UpdatedAt = time.Now()
}

func SetCPUInfo(info gos7.S7CpuInfo) {
	valuesMu.Lock()
	defer valuesMu.Unlock()
	cpu := info
	status.CPUInfo = &cpu
	status.UpdatedAt = time.Now()
}

func GetRuntimeStatus() RuntimeStatus {
	valuesMu.RLock()
	defer valuesMu.RUnlock()
	return status
}

func SetDashboardWatchList(dbNumber int32, addresses []string) {
	valuesMu.Lock()
	defer valuesMu.Unlock()
	dashboardDB = dbNumber
	dashboardVars = normalizeAddresses(addresses)
}

func GetActiveDashboardWatchList() []string {
	valuesMu.RLock()
	defer valuesMu.RUnlock()
	out := make([]string, len(dashboardVars))
	copy(out, dashboardVars)
	return out
}

func GetActiveDashboardDB() int32 {
	valuesMu.RLock()
	defer valuesMu.RUnlock()
	return dashboardDB
}

func SetValue(address, value, errText string) {
	key := normalizeAddress(address)
	if key == "" {
		return
	}
	valuesMu.Lock()
	defer valuesMu.Unlock()
	values[key] = ValueSnapshot{
		Address:   key,
		Value:     value,
		Error:     errText,
		UpdatedAt: time.Now(),
	}
}

func GetValue(address string) (ValueSnapshot, bool) {
	key := normalizeAddress(address)
	valuesMu.RLock()
	defer valuesMu.RUnlock()
	v, ok := values[key]
	return v, ok
}

func SetClient(client gos7.Client) {
	clientMu.Lock()
	defer clientMu.Unlock()
	plcClient = client
}

func ClearClient() {
	clientMu.Lock()
	defer clientMu.Unlock()
	plcClient = nil
}

func ReadValues(addresses []string) ([]ValueSnapshot, error) {
	clientMu.Lock()
	defer clientMu.Unlock()
	if plcClient == nil {
		return nil, fmt.Errorf("plc not connected")
	}
	cleanAddresses := normalizeAddresses(addresses)
	results := make([]ValueSnapshot, 0, len(cleanAddresses))
	for _, addr := range cleanAddresses {
		buff := make([]byte, 256)
		raw, err := plcClient.Read(addr, buff)
		if err != nil {
			SetValue(addr, "", err.Error())
			if v, ok := GetValue(addr); ok {
				results = append(results, v)
			}
			continue
		}
		v := fmt.Sprintf("%v", normalizeReadValue(addr, raw, buff))
		SetValue(addr, v, "")
		if value, ok := GetValue(addr); ok {
			results = append(results, value)
		}
	}
	now := time.Now()
	valuesMu.Lock()
	status.LastReadAt = &now
	if status.Connected && status.PLCState == "CONNECTED" {
		status.PLCState = "RUN"
	}
	status.UpdatedAt = now
	valuesMu.Unlock()
	return results, nil
}

var realAddr = regexp.MustCompile(`(?i)\.DBD[0-9]+$`)

func normalizeReadValue(address string, value any, raw []byte) any {
	if realAddr.MatchString(strings.TrimSpace(address)) {
		var h gos7.Helper
		return h.GetRealAt(raw, 0)
	}
	return value
}

func normalizeAddresses(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, item := range in {
		key := normalizeAddress(item)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func normalizeAddress(v string) string {
	return strings.ToUpper(strings.TrimSpace(v))
}
