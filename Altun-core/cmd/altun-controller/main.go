// altun-controller is the central coordinator that manages the Virtual Log,
// Policy Windows, and Protocol Reconfiguration (Algorithm 1). It exposes an
// HTTP API for the RL agent (Altun_RL) and operator to trigger switches.
package main

import (
	"crypto/ed25519"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wtwinlab/altun/vplane"
)

var (
	httpPort   = flag.Int("http-port", 8080, "HTTP API port for RL agent / operator")
	cftBin     = flag.String("cft-adapter", "./cft-adapter", "path to cft-adapter binary")
	bftBin     = flag.String("bft-adapter", "./bft-adapter", "path to bft-adapter binary")
	basePort   = flag.Int("base-grpc-port", 9100, "starting port for adapter gRPC servers")
	metaFile   = flag.String("meta-file", "vlog_meta.json", "virtual log metadata persistence file")
	timeoutSec = flag.Int("timeout", 30, "activation timeout in seconds (τ)")

	// Initial protocol to start with
	initProtocol = flag.String("init-protocol", "paxos", "initial consensus protocol")
	initMode     = flag.String("init-mode", "CFT", "initial fault mode (CFT or BFT)")

	// Forensic verifier (optional; empty = skip safety check at CFT boundary)
	verifierBin  = flag.String("verifier-bin", "", "path to forensic-verify binary (empty=skip)")
	forensicsLog = flag.String("forensics-log", "", "path to FORENSICS_LOG file (empty=skip)")
)

// Global controller state
var (
	vl       *vplane.VirtualLog
	rm       *vplane.ReconfigManager
	launcher *vplane.AdapterLauncher
)

// ReconfigRequest is the JSON body for POST /reconfig.
type ReconfigRequest struct {
	Trigger      string             `json:"trigger"`       // "intra" or "cross"
	Target       string             `json:"target"`        // target protocol name
	PolicyWindow *vplane.PolicyWindow `json:"policy_window"` // required for cross-family
	// Launch config overrides (optional)
	Mode        string `json:"mode"`         // "CFT" or "BFT" (for cross-family)
	ConfigPath  string `json:"config_path"`  // cluster config path
	BFTBinPath  string `json:"bft_bin_path"` // bft-server binary path
	Replicas    int    `json:"replicas"`
	Clients     int    `json:"clients"`
	BatchSize   int    `json:"batch_size"`
	PayloadSize int    `json:"payload_size"`
	RateLimit   int    `json:"rate_limit"`
	Duration    int    `json:"duration"`
	OutputDir   string `json:"output_dir"`
}

func main() {
	flag.Parse()

	// Generate ephemeral operator keypair for development
	pubKey, privKey, _ := ed25519.GenerateKey(nil)
	_ = privKey // would be loaded from config in production

	// Initialize components
	launcher = vplane.NewAdapterLauncher(*cftBin, *bftBin, *basePort)
	vl = vplane.NewWithJSONFile(*metaFile)
	timeout := time.Duration(*timeoutSec) * time.Second

	// Create initial policy window (self-signed for dev)
	initPolicy := &vplane.PolicyWindow{
		ID:        1,
		Mode:      *initMode,
		N:         4,
		F_CF:      1,
		F_BF:      1,
		Protocols: cftProtocols(),
	}
	if *initMode == "BFT" {
		initPolicy.Protocols = bftProtocols()
	}
	initPolicy.Sign(privKey)

	rm = vplane.NewReconfigManager(vl, launcher, initPolicy, pubKey, timeout)

	// Enable forensic verifier if paths are provided
	if *verifierBin != "" && *forensicsLog != "" {
		rm.SetVerifier(*verifierBin, *forensicsLog)
		fmt.Printf("altun-controller: forensic verifier enabled (%s)\n", *verifierBin)
	}

	// Launch initial adapter
	cfg := vplane.LaunchConfig{
		Protocol:   *initProtocol,
		Mode:       *initMode,
		InstanceID: 1,
	}
	grpcAddr, err := launcher.Launch(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to launch initial adapter: %v\n", err)
		os.Exit(1)
	}

	// Wait for adapter to be ready
	time.Sleep(3 * time.Second)

	// Register and activate in virtual log
	vl.RegisterInstance(vplane.InstanceMeta{
		ID:       1,
		Mode:     *initMode,
		Protocol: *initProtocol,
		GRPCAddr: grpcAddr,
	})
	if err := vl.Activate(1); err != nil {
		fmt.Fprintf(os.Stderr, "failed to activate initial instance: %v\n", err)
		os.Exit(1)
	}

	// HTTP API
	http.HandleFunc("/reconfig", handleReconfig)
	http.HandleFunc("/status", handleStatus)
	http.HandleFunc("/instances", handleInstances)

	go func() {
		addr := fmt.Sprintf(":%d", *httpPort)
		fmt.Printf("altun-controller: listening on %s\n", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			fmt.Fprintf(os.Stderr, "http error: %v\n", err)
		}
	}()

	// Block until shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh
	fmt.Println("altun-controller: shutting down")
	launcher.StopAll()
}

func handleReconfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var req ReconfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	trigger := vplane.IntraFamily
	if req.Trigger == "cross" {
		trigger = vplane.CrossFamily
	}

	mode := rm.CurrentPolicy().Mode
	if req.Mode != "" {
		mode = req.Mode
	}

	cfg := vplane.LaunchConfig{
		Protocol:    req.Target,
		Mode:        mode,
		ConfigPath:  req.ConfigPath,
		BFTBinPath:  req.BFTBinPath,
		Replicas:    req.Replicas,
		Clients:     req.Clients,
		BatchSize:   req.BatchSize,
		PayloadSize: req.PayloadSize,
		RateLimit:   req.RateLimit,
		Duration:    req.Duration,
		OutputDir:   req.OutputDir,
	}

	result, err := rm.Reconfigure(trigger, req.Target, req.PolicyWindow, cfg)
	if err != nil {
		resp := map[string]interface{}{"success": false, "error": err.Error()}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp := map[string]interface{}{
		"success":      true,
		"new_instance": result,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	info, err := vl.CurrentInfo(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	status, _ := vl.CurrentClient().LastCommitted(ctx)
	resp := map[string]interface{}{
		"instance": info,
		"commit":   status,
		"policy":   rm.CurrentPolicy(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleInstances(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vl.Instances())
}

func cftProtocols() []string {
	return []string{"paxos", "epaxos", "vpaxos", "wpaxos"}
}

func bftProtocols() []string {
	return []string{"chainedhotstuff", "fasthotstuff", "mirbft", "pbft"}
}
