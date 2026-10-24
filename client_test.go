package powerwall

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSkipLoginHTTPScheme(t *testing.T) {
	var loginCalls int32

	mux := http.NewServeMux()
	mux.HandleFunc("/api/login/Basic", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&loginCalls, 1)
		http.Error(w, "login not expected", http.StatusInternalServerError)
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"din":              "test-din",
			"start_time":       "2020-01-01 00:00:00 +0000",
			"up_time_seconds":  "1h2m3s",
			"is_new":           false,
			"version":          "1.0",
			"git_hash":         "",
			"commission_count": 0,
			"device_type":      "test",
			"sync_type":        "test",
		})
	})
	mux.HandleFunc("/api/meters/aggregates", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"site": map[string]any{
				"instant_power":   1.5,
				"energy_imported": 1000,
				"energy_exported": 2000,
				"i_a_current":     1,
				"i_b_current":     2,
				"i_c_current":     3,
			},
		})
	})
	mux.HandleFunc("/api/system_status", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"nominal_full_pack_energy":      27000,
			"max_apparent_power":            9200,
			"inverter_nominal_usable_power": 9200,
		})
	})
	mux.HandleFunc("/api/system_status/soe", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"percentage": 50})
	})
	mux.HandleFunc("/api/operation", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"real_mode":              "autonomous",
			"backup_reserve_percent": 20,
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewClient(
		srv.Listener.Addr().String(),
		"",
		"",
		WithScheme("http"),
		WithSkipLogin(),
		WithHttpClient(srv.Client()),
	)

	if err := client.DoLogin(); err != nil {
		t.Fatalf("DoLogin: %v", err)
	}
	client.SetAuthToken("ignored")
	if token := client.GetAuthToken(); token != "" {
		t.Fatalf("unexpected auth token: %q", token)
	}

	status, err := client.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if status.Din != "test-din" || status.Version != "1.0" {
		t.Fatalf("unexpected status: %+v", status)
	}

	aggregates, err := client.GetMetersAggregates()
	if err != nil {
		t.Fatalf("GetMetersAggregates: %v", err)
	}
	if got := aggregates["site"].InstantPower; got != 1.5 {
		t.Fatalf("unexpected instant power: %v", got)
	}

	systemStatus, err := client.GetSystemStatus()
	if err != nil {
		t.Fatalf("GetSystemStatus: %v", err)
	}
	if systemStatus.NominalFullPackEnergy != 27000 {
		t.Fatalf("unexpected system status: %+v", systemStatus)
	}

	soe, err := client.GetSOE()
	if err != nil {
		t.Fatalf("GetSOE: %v", err)
	}
	if soe.Percentage != 50 {
		t.Fatalf("unexpected soe: %+v", soe)
	}

	operation, err := client.GetOperation()
	if err != nil {
		t.Fatalf("GetOperation: %v", err)
	}
	if operation.BackupReservePercent != 20 {
		t.Fatalf("unexpected operation: %+v", operation)
	}

	if calls := atomic.LoadInt32(&loginCalls); calls != 0 {
		t.Fatalf("unexpected login calls: %d", calls)
	}
}

func TestUnsupportedScheme(t *testing.T) {
	client := NewClient("127.0.0.1:1", "", "", WithScheme("ftp"), WithSkipLogin())
	_, err := client.GetStatus()
	if err == nil || !strings.Contains(err.Error(), "unsupported scheme") {
		t.Fatalf("expected unsupported scheme error, got %v", err)
	}
}
