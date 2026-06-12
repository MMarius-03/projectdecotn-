// Verificare diferente deconturi - server web local + scanare Excel.
//
// Executabilul sta in folderul SMB, dar fiecare utilizator il ruleaza local.
// Interfata se deschide in browser pe 127.0.0.1, fisierele se citesc/scriu pe share.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	cfg     Config
	cfgMu   sync.RWMutex
	verifMu sync.Mutex // o singura verificare in aceasta instanta odata
)

func main() {
	cfg = loadConfig()

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/state", handleState)
	mux.HandleFunc("/api/setpath", handleSetPath)
	mux.HandleFunc("/api/verify", handleVerify)
	mux.HandleFunc("/api/open", handleOpen)

	addr := fmt.Sprintf("127.0.0.1:%d", currentPort())
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		// portul cerut e ocupat -> alege automat unul liber.
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			log.Fatalf("Nu pot porni serverul local: %v", err)
		}
	}
	url := "http://" + ln.Addr().String()
	fmt.Println("Verificare diferente deconturi ruleaza la:", url)
	fmt.Println("Inchide aceasta fereastra ca sa opresti aplicatia.")

	go func() {
		time.Sleep(400 * time.Millisecond)
		openBrowser(url)
	}()

	if err := http.Serve(ln, mux); err != nil {
		log.Fatal(err)
	}
}

func currentPort() int {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	if cfg.Port < 0 {
		return 0
	}
	return cfg.Port
}

func getCfg() Config {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	return cfg
}

// ---------------------------------------------------------------------------
// handlers
// ---------------------------------------------------------------------------

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

func handleState(w http.ResponseWriter, r *http.Request) {
	c := getCfg()
	months, err := listMonths(c.BasePath)
	resp := map[string]any{
		"base_path":     c.BasePath,
		"report_folder": c.ReportFolder,
		"months":        months,
	}
	if err != nil {
		resp["error"] = err.Error()
	}
	writeJSON(w, resp)
}

func handleSetPath(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BasePath string `json:"base_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "json invalid", http.StatusBadRequest)
		return
	}
	body.BasePath = strings.TrimSpace(body.BasePath)
	if body.BasePath == "" {
		http.Error(w, "cale goala", http.StatusBadRequest)
		return
	}
	cfgMu.Lock()
	cfg.BasePath = body.BasePath
	c := cfg
	cfgMu.Unlock()
	_ = saveConfig(c)
	writeJSON(w, map[string]any{"ok": true})
}

// listMonths intoarce subfolderele de luna din folderul anului, sortate.
func listMonths(base string) ([]string, error) {
	info, err := os.Stat(base)
	if err != nil {
		return nil, fmt.Errorf("cale inaccesibila")
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("calea nu este un folder")
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, fmt.Errorf("nu pot citi folderul")
	}
	var months []string
	for _, e := range entries {
		if e.IsDir() {
			months = append(months, e.Name())
		}
	}
	sort.Strings(months)
	return months, nil
}

// handleVerify ruleaza scanarea si trimite progres prin SSE.
func handleVerify(w http.ResponseWriter, r *http.Request) {
	month := strings.TrimSpace(r.URL.Query().Get("month"))
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming nesuportat", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	send := func(event string, payload any) {
		b, _ := json.Marshal(payload)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
		flusher.Flush()
	}
	sendRaw := func(event, msg string) {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, msg)
		flusher.Flush()
	}

	c := getCfg()
	if month == "" {
		send("done", map[string]any{"error": "Nicio luna selectata."})
		return
	}
	monthDir := filepath.Join(c.BasePath, month)
	if info, err := os.Stat(monthDir); err != nil || !info.IsDir() {
		send("done", map[string]any{"error": "Folderul lunii nu este accesibil."})
		return
	}

	// o singura verificare per instanta.
	if !verifMu.TryLock() {
		send("done", map[string]any{"error": "O verificare este deja in curs in aceasta aplicatie."})
		return
	}
	defer verifMu.Unlock()

	// avertizare daca altcineva verifica aceeasi luna pe share.
	if warn := checkLock(monthDir, c.ReportFolder); warn != "" {
		sendRaw("warn", warn)
	}
	lockPath, _ := acquireLock(monthDir, c.ReportFolder)
	defer releaseLock(lockPath)

	// progres cu rate-limit (max ~10/sec) ca sa nu inundam clientul.
	var lastTick time.Time
	progress := func(done, total, withDif int) {
		if time.Since(lastTick) < 100*time.Millisecond && done < total {
			return
		}
		lastTick = time.Now()
		send("progress", map[string]any{"done": done, "total": total, "with_dif": withDif})
	}

	sum := scanMonth(monthDir, c.ReportFolder, c.Workers, progress)

	result := map[string]any{
		"checked":          sum.Checked,
		"rows":             len(sum.Diffs),
		"files_with_dif":   sum.FilesWithDif,
		"ignored":          sum.IgnoredOther,
		"unprocessed":      sum.Unprocessed,
		"incomplete":       sum.Incomplete,
		"incomplete_files": sum.IncompleteFiles,
		"errors":           sum.Errors,
		"report":           "",
		"folder":           filepath.Join(monthDir, c.ReportFolder),
	}

	if len(sum.Diffs) > 0 || len(sum.IncompleteFiles) > 0 {
		path, err := writeReport(monthDir, c.ReportFolder, month, sum.Diffs, c.DifRedThresholdPct/100.0, c.DifRedMinKg, sum.IncompleteFiles)
		if err != nil {
			result["error"] = err.Error()
		} else {
			result["report"] = path
		}
	}
	send("done", result)
}

// handleOpen deschide un fisier sau folder cu aplicatia implicita din Windows.
func handleOpen(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "cale lipsa", http.StatusBadRequest)
		return
	}
	if err := openPath(path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func openBrowser(url string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func openPath(path string) error {
	// "start" cu titlu gol gestioneaza corect caile cu spatii.
	return exec.Command("cmd", "/c", "start", "", path).Start()
}
