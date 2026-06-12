// Probe SMB - test minimal de acces la share-ul cu deconturile.
//
// Rulare:
//   go run .                  -> foloseste calea implicita din plan
//   go run . "\\\\server\\cale" -> testeaza o cale data
//
// Sau compilat:
//   go build -o probe.exe .
//   probe.exe "\\192.168.2.121\public\Departamente\Contabilitate\aNote Cantar\Note de cantar\2026"
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const defaultPath = `\\192.168.2.121\public\Departamente\Contabilitate\aNote Cantar\Note de cantar\2026`

func main() {
	base := defaultPath
	if len(os.Args) > 1 && strings.TrimSpace(os.Args[1]) != "" {
		base = os.Args[1]
	}

	fmt.Println("=== Probe acces SMB ===")
	fmt.Printf("Utilizator Windows : %s\n", os.Getenv("USERNAME"))
	fmt.Printf("Calculator         : %s\n", os.Getenv("COMPUTERNAME"))
	fmt.Printf("Cale testata       : %s\n\n", base)

	// 1. Exista calea?
	start := time.Now()
	info, err := os.Stat(base)
	if err != nil {
		fmt.Printf("[EROARE] Calea nu este accesibila: %v\n", err)
		fmt.Println("\nVerifica: VPN/retea, drepturi de citire, sau calea exacta.")
		pause()
		return
	}
	fmt.Printf("[OK] Cale accesibila in %v\n", time.Since(start).Round(time.Millisecond))
	if !info.IsDir() {
		fmt.Println("[EROARE] Calea exista dar NU este un folder.")
		pause()
		return
	}

	// 2. Citire continut (subfoldere de luna + numar fisiere xlsx).
	entries, err := os.ReadDir(base)
	if err != nil {
		fmt.Printf("[EROARE] Nu pot citi folderul: %v\n", err)
		pause()
		return
	}

	var months []string
	for _, e := range entries {
		if e.IsDir() {
			months = append(months, e.Name())
		}
	}
	sort.Strings(months)

	fmt.Printf("\n[OK] Citire reusita. %d subfoldere gasite:\n", len(months))
	for _, m := range months {
		sub := filepath.Join(base, m)
		count := countXlsx(sub)
		fmt.Printf("   - %-25s (%d fisiere .xlsx)\n", m, count)
	}

	// 3. Test de SCRIERE intr-un folder temporar de raport in prima luna.
	if len(months) > 0 {
		testWrite(filepath.Join(base, months[0]))
	} else {
		fmt.Println("\n[ATENTIE] Nu exista subfoldere de luna; sar peste testul de scriere.")
	}

	fmt.Println("\n=== Gata. ===")
	pause()
}

// countXlsx numara fisierele .xlsx reale (ignora temporarele ~$).
func countXlsx(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return -1
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, "~$") {
			continue
		}
		if strings.EqualFold(filepath.Ext(name), ".xlsx") {
			n++
		}
	}
	return n
}

// testWrite verifica daca putem crea/sterge un fisier (drepturi de scriere).
func testWrite(monthDir string) {
	reportDir := filepath.Join(monthDir, "Rapoarte_Diferente")
	fmt.Printf("\n[TEST SCRIERE] in: %s\n", reportDir)

	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		fmt.Printf("[EROARE] Nu pot crea folderul de raport: %v\n", err)
		return
	}
	testFile := filepath.Join(reportDir, fmt.Sprintf("_probe_%d.tmp", time.Now().UnixNano()))
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		fmt.Printf("[EROARE] Nu am drept de scriere: %v\n", err)
		return
	}
	_ = os.Remove(testFile)
	fmt.Println("[OK] Scriere si stergere reusite (drepturi de scriere confirmate).")
}

// pause asteapta Enter ca fereastra sa nu se inchida la dublu-click.
func pause() {
	fmt.Print("\nApasa Enter pentru a inchide...")
	fmt.Scanln()
}
