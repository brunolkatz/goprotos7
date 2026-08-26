package plc_runtime

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/robinson/gos7"
)

type ValueSnapshot struct {
	Address   string    `json:"address"`
	Value     string    `json:"value"`
	Error     string    `json:"error,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RuntimeStatus struct {
	Connected bool      `json:"connected"`
	Address   string    `json:"address,omitempty"`
	Status    string    `json:"status,omitempty"`
	LastError string    `json:"last_error,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

var (
	valuesMu      sync.RWMutex
	values        = map[string]ValueSnapshot{}
	runtimeStatus = RuntimeStatus{Connected: false, Status: "DISCONNECTED"}
	dashboardDB   int32
	dashboardVars []string
	clientMu      sync.Mutex
	plcClient     gos7.Client
)

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

func SetConnected(address string) {
	valuesMu.Lock()
	defer valuesMu.Unlock()
	runtimeStatus.Connected = true
	runtimeStatus.Address = strings.TrimSpace(address)
	if runtimeStatus.Status == "" || runtimeStatus.Status == "DISCONNECTED" {
		runtimeStatus.Status = "UNKNOWN"
	}
	runtimeStatus.LastError = ""
	runtimeStatus.UpdatedAt = time.Now()
}

func SetDisconnected(errText string) {
	valuesMu.Lock()
	defer valuesMu.Unlock()
	runtimeStatus.Connected = false
	runtimeStatus.Status = "DISCONNECTED"
	runtimeStatus.Address = ""
	runtimeStatus.LastError = strings.TrimSpace(errText)
	runtimeStatus.UpdatedAt = time.Now()
}

func SetPLCStatus(status string) {
	normalized := strings.ToUpper(strings.TrimSpace(status))
	if normalized == "" {
		return
	}
	valuesMu.Lock()
	defer valuesMu.Unlock()
	runtimeStatus.Status = normalized
	runtimeStatus.UpdatedAt = time.Now()
}

func GetRuntimeStatus() RuntimeStatus {
	valuesMu.RLock()
	defer valuesMu.RUnlock()
	return runtimeStatus
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
