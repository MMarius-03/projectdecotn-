package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// ScanSummary aduna rezultatul verificarii unei luni intregi.
type ScanSummary struct {
	TotalFiles      int       // fisiere de nota gasite in folderele de zi
	Checked         int       // fisiere citite cu succes care aveau tabel de livrare
	Unprocessed     int       // fisiere fara tabel de livrare (alt format)
	Incomplete      int       // note cu CANT LIVRAT necompletat peste tot (excluse din raport)
	Errors          int       // fisiere corupte / blocate / necitibile
	IgnoredOther    int       // .xlsx aflate in afara folderelor de zi (casa/jurnal/stoc)
	FilesWithDif    int       // fisiere care au cel putin o diferenta
	Diffs           []DiffRow // toate diferentele gasite
	ErrorFiles      []string  // numele fisierelor cu eroare, pentru sumar
	IncompleteFiles []string  // numele notelor incomplete, pentru sumar
}

// collectNoteFiles enumera fisierele de nota dintr-o luna.
// Intra DOAR in folderele de zi (nume tip ZZ.LL.AAAA). Numara separat
// fisierele .xlsx din alte foldere (casa/jurnal/stoc, Rapoarte) ca "ignorate".
func collectNoteFiles(monthDir, reportFolder string) (notes []string, ignoredOther int) {
	entries, err := os.ReadDir(monthDir)
	if err != nil {
		return nil, 0
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			if isDayFolder(name) {
				notes = append(notes, listDayXlsx(filepath.Join(monthDir, name))...)
			} else if !strings.EqualFold(name, reportFolder) {
				// folder care nu e zi si nu e folderul de rapoarte: numara xlsx-urile ca ignorate.
				ignoredOther += countXlsxShallow(filepath.Join(monthDir, name))
			}
			continue
		}
		// fisier .xlsx direct in luna (nu in folder de zi) -> ignorat.
		if isXlsx(name) {
			ignoredOther++
		}
	}
	sort.Strings(notes)
	return notes, ignoredOther
}

// listDayXlsx returneaza .xlsx-urile reale dintr-un folder de zi.
func listDayXlsx(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if isXlsx(e.Name()) {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}

func countXlsxShallow(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && isXlsx(e.Name()) {
			n++
		}
	}
	return n
}

// isXlsx: .xlsx real, exclude temporarele Excel ~$.
func isXlsx(name string) bool {
	if strings.HasPrefix(name, "~$") {
		return false
	}
	return strings.EqualFold(filepath.Ext(name), ".xlsx")
}

// ProgressFn este chemata pe masura ce fisierele sunt verificate.
type ProgressFn func(done, total, filesWithDif int)

// scanMonth verifica toate notele dintr-o luna folosind un pool de workeri.
func scanMonth(monthDir, reportFolder string, workers int, progress ProgressFn) ScanSummary {
	notes, ignored := collectNoteFiles(monthDir, reportFolder)
	sum := ScanSummary{TotalFiles: len(notes), IgnoredOther: ignored}
	if len(notes) == 0 {
		return sum
	}
	if workers < 1 {
		workers = 1
	}

	type job struct {
		idx  int
		path string
	}
	results := make([]FileResult, len(notes))
	jobs := make(chan job)
	var wg sync.WaitGroup
	var done, withDif int64
	total := len(notes)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				r := scanFile(j.path, filepath.Base(j.path))
				results[j.idx] = r
				if r.Err == nil && r.Processed && len(r.Diffs) > 0 {
					atomic.AddInt64(&withDif, 1)
				}
				d := atomic.AddInt64(&done, 1)
				if progress != nil {
					progress(int(d), total, int(atomic.LoadInt64(&withDif)))
				}
			}
		}()
	}

	go func() {
		for i, p := range notes {
			jobs <- job{idx: i, path: p}
		}
		close(jobs)
	}()
	wg.Wait()

	// agregare in ordine stabila (dupa cale, apoi rand).
	for _, r := range results {
		switch {
		case r.Err != nil:
			sum.Errors++
			sum.ErrorFiles = append(sum.ErrorFiles, r.Fisier)
		case !r.Processed:
			sum.Unprocessed++
		case r.Incomplete:
			sum.Incomplete++
			sum.IncompleteFiles = append(sum.IncompleteFiles, r.Fisier)
		default:
			sum.Checked++
			if len(r.Diffs) > 0 {
				sum.FilesWithDif++
				sum.Diffs = append(sum.Diffs, r.Diffs...)
			}
		}
	}
	return sum
}
