package plc_runtime

import (
	"strings"
	"sync"
	"time"
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
