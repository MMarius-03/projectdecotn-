package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	lockName  = ".verificare.lock"
	lockStale = 10 * time.Minute
)

// lockData descrie cine are o verificare in curs.
type lockData struct {
	User     string    `json:"user"`
	Computer string    `json:"computer"`
	PID      int       `json:"pid"`
	Started  time.Time `json:"started"`
}

// checkLock returneaza un mesaj de avertizare daca altcineva pare sa verifice
// aceeasi luna chiar acum. Returneaza "" daca e liber.
func checkLock(monthDir, reportFolder string) string {
	p := filepath.Join(monthDir, reportFolder, lockName)
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	var ld lockData
	if json.Unmarshal(b, &ld) != nil {
		return ""
	}
	if time.Since(ld.Started) > lockStale {
		return "" // lock vechi, abandonat
	}
	me := getenvDefault("COMPUTERNAME", "") + "/" + getenvDefault("USERNAME", "")
	other := ld.Computer + "/" + ld.User
	if other == me {
		return ""
	}
	return "Atentie: " + ld.User + " (" + ld.Computer + ") pare sa verifice aceasta luna acum. Poti continua oricum."
}

// acquireLock scrie lock-ul nostru. Returneaza calea pentru eliberare.
func acquireLock(monthDir, reportFolder string) (string, error) {
	dir := filepath.Join(monthDir, reportFolder)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(dir, lockName)
	ld := lockData{
		User:     getenvDefault("USERNAME", "user"),
		Computer: getenvDefault("COMPUTERNAME", "pc"),
		PID:      os.Getpid(),
		Started:  time.Now(),
	}
	b, _ := json.MarshalIndent(ld, "", "  ")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		return "", err
	}
	return p, nil
}

func releaseLock(path string) {
	if path != "" {
		_ = os.Remove(path)
	}
}
