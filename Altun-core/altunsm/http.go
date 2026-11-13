package altunsm

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// ImportReq is the body of POST /altun/import.
type ImportReq struct {
	NewSeg uint64    `json:"new_seg"`
	Snap   *Snapshot `json:"snap"`
}

// RegisterHTTP installs the out-of-band admin endpoints of a replica:
//
//	GET  /altun/status    replica status (state, segment, table)
//	GET  /altun/snapshot  state at the last seal plus its digest
//	POST /altun/import    stage a snapshot for a new segment
//	GET  /altun/attest    signed attestation ?kind=seal|start&seg=K (409 until the replica reached that point)
func (m *SM) RegisterHTTP(mux *http.ServeMux) {
	mux.HandleFunc("/altun/attest", func(w http.ResponseWriter, r *http.Request) {
		seg, _ := strconv.ParseUint(r.URL.Query().Get("seg"), 10, 64)
		a, err := m.Attest(r.URL.Query().Get("kind"), seg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		_ = json.NewEncoder(w).Encode(a)
	})
	mux.HandleFunc("/altun/status", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(m.Status())
	})
	mux.HandleFunc("/altun/snapshot", func(w http.ResponseWriter, r *http.Request) {
		s, d, err := m.SealedSnapshot()
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		if want := r.URL.Query().Get("seg"); want != "" {
			if v, _ := strconv.ParseUint(want, 10, 64); v != s.Seg {
				http.Error(w, "sealed segment differs", http.StatusConflict)
				return
			}
		}
		w.Header().Set("Altun-Digest", d)
		_ = json.NewEncoder(w).Encode(s)
	})
	mux.HandleFunc("/altun/import", func(w http.ResponseWriter, r *http.Request) {
		var req ImportReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Snap == nil {
			http.Error(w, "bad import", http.StatusBadRequest)
			return
		}
		d, err := m.Import(req.Snap, req.NewSeg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		_, _ = w.Write([]byte(d))
	})
}
