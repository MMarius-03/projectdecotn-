// decont-diag - unealta de diagnostic pentru share-ul SMB cu deconturile.
//
// Scop: intelegem cum se comporta share-ul ca sa decidem modelul de deploy
// (un singur utilizator / doi / mai multi). Testeaza acces, concurenta la
// citire, fisiere blocate de Excel, contentie la scriere, lock de coordonare
// si viteza de citire peste retea.
//
// Structura reala a share-ului (4 niveluri):
//   2026\              an
//     06.IUNIE\        luna  (NN.NUMELUNA)
//       01.06.2026\    zi    (ZZ.LL.AAAA)
//         *.xlsx       notele de cantar
//
// Comenzi:
//   decont-diag info   [an]               acces + luni + zile + numar fisiere + fisiere deschise
//   decont-diag tree   [cale] [adancime]  arata structura reala de foldere (descoperire)
//   decont-diag bench  [luna] [N]         citeste recursiv toate .xlsx, secvential vs paralel
//   decont-diag write  [luna] [N]         test contentie: N "utilizatori" scriu rapoarte simultan
//   decont-diag lock   status [luna]      arata cine are verificarea in curs (daca cineva)
//   decont-diag lock   hold   [luna] [sec] tine un lock activ N secunde (pt test intre instante)
//   decont-diag open   <fisier.xlsx>      incearca sa deschida un fisier (chiar daca e deschis in Excel)
//
// "luna" = folderul lunii (ex ...\2026\06.IUNIE). Cautarea fisierelor e recursiva
// (intra in folderele de zi). Fara argumente foloseste calea implicita din plan.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xuri/excelize/v2"
)

const defaultBase = `\\192.168.2.121\public\Departamente\Contabilitate\aNote Cantar\Note de cantar\2026`

const (
	reportFolder = "Rapoarte_Diferente"
	lockName     = ".verificare.lock"
	lockStale    = 10 * time.Minute // un lock mai vechi de atat e considerat abandonat
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		return
	}
	switch strings.ToLower(args[0]) {
	case "info":
		cmdInfo(arg(args, 1, defaultBase))
	case "tree":
		cmdTree(arg(args, 1, defaultBase), atoiDef(arg(args, 2, "3"), 3))
	case "bench":
		cmdBench(arg(args, 1, ""), atoiDef(arg(args, 2, "4"), 4))
	case "write":
		cmdWrite(arg(args, 1, ""), atoiDef(arg(args, 2, "5"), 5))
	case "lock":
		cmdLock(args[1:])
	case "open":
		cmdOpen(arg(args, 1, ""))
	default:
		usage()
	}
}

func usage() {
	fmt.Print(`decont-diag - diagnostic share SMB deconturi

Structura: 2026\ -> 06.IUNIE\ -> 01.06.2026\ -> *.xlsx  (an/luna/zi/fisiere)

  decont-diag info   [an]                 acces, luni, zile, numar fisiere, fisiere deschise
  decont-diag tree   [cale] [adancime]    arata structura reala de foldere (descoperire)
  decont-diag bench  [folderLuna] [N]     viteza citire recursiva: secvential vs N workeri
  decont-diag write  [folderLuna] [N]     N utilizatori scriu rapoarte simultan (contentie)
  decont-diag lock   status [folderLuna]  cine are verificarea in curs
  decont-diag lock   hold   [folderLuna] [sec]   tine lock N sec (test intre 2 instante)
  decont-diag open   <fisier.xlsx>        deschide un fisier chiar daca e deschis in Excel

Fara cale, foloseste implicit:
  ` + defaultBase + "\n")
	pause()
}

// ---------------------------------------------------------------------------
// info
// ---------------------------------------------------------------------------

func cmdInfo(base string) {
	header("INFO", base)

	t0 := time.Now()
	info, err := os.Stat(base)
	if err != nil {
		fail("Cale inaccesibila", err)
		return
	}
	okf("Cale accesibila in %v", time.Since(t0).Round(time.Millisecond))
	if !info.IsDir() {
		fail("Calea nu e folder", nil)
		return
	}

	entries, err := os.ReadDir(base)
	if err != nil {
		fail("Nu pot citi folderul", err)
		return
	}
	var months []string
	for _, e := range entries {
		if e.IsDir() {
			months = append(months, e.Name())
		}
	}
	sort.Strings(months)
	okf("%d subfoldere de luna", len(months))

	totalFiles, totalOpen, totalDays := 0, 0, 0
	var totalBytes int64
	for _, m := range months {
		sub := filepath.Join(base, m)
		st := walkStats(sub)
		totalFiles += st.files
		totalOpen += st.open
		totalBytes += st.bytes
		totalDays += st.dayDirs
		note := ""
		if st.open > 0 {
			note = fmt.Sprintf("  [%d deschise in Excel]", st.open)
		}
		fmt.Printf("   - %-12s %3d zile  %4d fisiere  %9s%s\n",
			m, st.dayDirs, st.files, human(st.bytes), note)
	}
	fmt.Println()
	okf("TOTAL: %d luni, %d foldere de zi, %d fisiere .xlsx, %s, %d deschise in Excel",
		len(months), totalDays, totalFiles, human(totalBytes), totalOpen)

	if len(months) > 0 {
		writeProbe(filepath.Join(base, months[len(months)-1]))
	}
	done()
}

// dirStats aduna statistici recursive sub o luna.
type dirStats struct {
	files   int   // .xlsx reale (exclude folderul de rapoarte si ~$)
	open    int   // fisiere ~$ (deschise in Excel)
	dayDirs int   // foldere directe de zi
	bytes   int64 // dimensiune totala xlsx
}

// walkStats parcurge recursiv o luna: numara zilele, fisierele, cele deschise.
func walkStats(monthDir string) dirStats {
	var st dirStats
	if entries, err := os.ReadDir(monthDir); err == nil {
		for _, e := range entries {
			if e.IsDir() && !strings.EqualFold(e.Name(), reportFolder) {
				st.dayDirs++
			}
		}
	}
	_ = filepath.WalkDir(monthDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.EqualFold(d.Name(), reportFolder) {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, "~$") {
			st.open++
			return nil
		}
		if strings.EqualFold(filepath.Ext(name), ".xlsx") {
			st.files++
			if fi, err := d.Info(); err == nil {
				st.bytes += fi.Size()
			}
		}
		return nil
	})
	return st
}

// cmdTree afiseaza structura reala de foldere pana la o adancime data.
func cmdTree(root string, maxDepth int) {
	header("TREE", root)
	if _, err := os.Stat(root); err != nil {
		fail("Cale inaccesibila", err)
		return
	}
	fmt.Printf("%s\n", root)
	printed := 0
	var rec func(dir string, depth int, prefix string)
	rec = func(dir string, depth int, prefix string) {
		if depth > maxDepth || printed > 400 {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			fmt.Printf("%s  [eroare: %v]\n", prefix, err)
			return
		}
		var dirs []string
		files, open := 0, 0
		for _, e := range entries {
			if e.IsDir() {
				dirs = append(dirs, e.Name())
			} else if strings.HasPrefix(e.Name(), "~$") {
				open++
			} else if strings.EqualFold(filepath.Ext(e.Name()), ".xlsx") {
				files++
			}
		}
		sort.Strings(dirs)
		if files > 0 || open > 0 {
			tag := fmt.Sprintf("%d xlsx", files)
			if open > 0 {
				tag += fmt.Sprintf(", %d deschise", open)
			}
			fmt.Printf("%s   (%s)\n", prefix, tag)
		}
		for _, name := range dirs {
			printed++
			fmt.Printf("%s├─ %s\\\n", prefix, name)
			rec(filepath.Join(dir, name), depth+1, prefix+"│  ")
		}
	}
	rec(root, 1, "")
	done()
}

func writeProbe(monthDir string) {
	rd := filepath.Join(monthDir, reportFolder)
	fmt.Printf("\n[TEST SCRIERE] %s\n", rd)
	if err := os.MkdirAll(rd, 0o755); err != nil {
		fail("Nu pot crea folderul de raport", err)
		return
	}
	tf := filepath.Join(rd, fmt.Sprintf("_probe_%d.tmp", time.Now().UnixNano()))
	if err := os.WriteFile(tf, []byte("test"), 0o644); err != nil {
		fail("Fara drept de scriere", err)
		return
	}
	_ = os.Remove(tf)
	okf("Scriere + stergere reusite (am drepturi de scriere)")
}

// ---------------------------------------------------------------------------
// bench - viteza de citire secventiala vs paralela
// ---------------------------------------------------------------------------

func cmdBench(monthDir string, workers int) {
	if monthDir == "" {
		fail("Lipseste folderul lunii. Ex: decont-diag bench \"\\\\server\\...\\2026\\06.Iunie\"", nil)
		return
	}
	header("BENCH", monthDir)
	files := listXlsx(monthDir)
	if len(files) == 0 {
		fail("Niciun fisier .xlsx in folder", nil)
		return
	}
	okf("%d fisiere de citit", len(files))

	// 1) Secvential.
	fmt.Println("\n-> Citire SECVENTIALA (1 fisier pe rand)...")
	t0 := time.Now()
	rows1, errs1 := readAll(files, 1)
	d1 := time.Since(t0)
	report("secvential", d1, len(files), rows1, errs1)

	// 2) Paralel.
	fmt.Printf("\n-> Citire PARALELA (%d workeri)...\n", workers)
	t1 := time.Now()
	rows2, errs2 := readAll(files, workers)
	d2 := time.Since(t1)
	report("paralel", d2, len(files), rows2, errs2)

	fmt.Println()
	if d2 > 0 && d1 > 0 {
		speedup := float64(d1) / float64(d2)
		okf("Paralelismul aduce x%.1f (%v -> %v)", speedup, d1.Round(time.Millisecond), d2.Round(time.Millisecond))
		if speedup < 1.3 {
			fmt.Println("   => Reteaua e gatuirea; concurenta NU ajuta mult. Citire secventiala e ok.")
		} else {
			fmt.Println("   => Concurenta ajuta clar; merita workeri paraleli in app.")
		}
	}
	done()
}

// readAll citeste fisierele cu un pool de workeri si numara randurile totale.
func readAll(files []string, workers int) (totalRows int64, errCount int64) {
	if workers < 1 {
		workers = 1
	}
	ch := make(chan string)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range ch {
				n, err := readOne(p)
				if err != nil {
					atomic.AddInt64(&errCount, 1)
					continue
				}
				atomic.AddInt64(&totalRows, int64(n))
			}
		}()
	}
	for _, p := range files {
		ch <- p
	}
	close(ch)
	wg.Wait()
	return
}

// readOne deschide un xlsx si parcurge toate randurile (forteaza parsarea reala).
func readOne(path string) (int, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	total := 0
	for _, s := range f.GetSheetList() {
		rows, err := f.GetRows(s)
		if err != nil {
			continue
		}
		total += len(rows)
	}
	return total, nil
}

func report(label string, d time.Duration, files int, rows, errs int64) {
	perFile := d / time.Duration(max(files, 1))
	fmt.Printf("   %-11s %v total | %v/fisier | %d randuri | %d erori\n",
		label, d.Round(time.Millisecond), perFile.Round(time.Millisecond), rows, errs)
}

// ---------------------------------------------------------------------------
// write - contentie la scriere (multi utilizatori scriu rapoarte simultan)
// ---------------------------------------------------------------------------

func cmdWrite(monthDir string, n int) {
	if monthDir == "" {
		fail("Lipseste folderul lunii.", nil)
		return
	}
	header("WRITE", monthDir)
	rd := filepath.Join(monthDir, reportFolder)
	if err := os.MkdirAll(rd, 0o755); err != nil {
		fail("Nu pot crea folderul de raport", err)
		return
	}
	okf("Simulez %d utilizatori care scriu rapoarte in acelasi timp", n)

	var wg sync.WaitGroup
	var okCnt, errCnt int64
	t0 := time.Now()
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("_test_user%02d_%s_%d.xlsx", id, hostUser(), time.Now().UnixNano())
			p := filepath.Join(rd, name)
			if err := writeMiniReport(p, id); err != nil {
				atomic.AddInt64(&errCnt, 1)
				fmt.Printf("   [user%02d] EROARE: %v\n", id, err)
				return
			}
			atomic.AddInt64(&okCnt, 1)
			_ = os.Remove(p) // curatam fisierul de test
		}(i)
	}
	wg.Wait()

	fmt.Println()
	okf("Scrieri reusite: %d / %d  in %v", okCnt, n, time.Since(t0).Round(time.Millisecond))
	if errCnt == 0 {
		fmt.Println("   => Numele unice (user+timestamp) evita coliziunile. Scriere multi-user OK.")
	} else {
		fmt.Printf("   => ATENTIE: %d scrieri au esuat; investigam drepturile/blocajele.\n", errCnt)
	}
	done()
}

// writeMiniReport scrie un xlsx mic real (ca sa testam scrierea ca in productie).
func writeMiniReport(path string, id int) error {
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetCellValue("Sheet1", "A1", "Test")
	_ = f.SetCellValue("Sheet1", "A2", id)
	return f.SaveAs(path)
}

// ---------------------------------------------------------------------------
// lock - mecanism de coordonare "o verificare in curs"
// ---------------------------------------------------------------------------

type lockData struct {
	User     string    `json:"user"`
	Computer string    `json:"computer"`
	PID      int       `json:"pid"`
	Started  time.Time `json:"started"`
}

func cmdLock(args []string) {
	sub := strings.ToLower(arg(args, 0, "status"))
	monthDir := arg(args, 1, "")
	if monthDir == "" {
		fail("Lipseste folderul lunii. Ex: decont-diag lock status \"...\\06.Iunie\"", nil)
		return
	}
	lockPath := filepath.Join(monthDir, reportFolder, lockName)

	switch sub {
	case "status":
		header("LOCK STATUS", monthDir)
		ld, fresh, err := readLock(lockPath)
		if err != nil {
			okf("Niciun lock activ (nimeni nu verifica acum). Liber.")
		} else if fresh {
			fmt.Printf("[OCUPAT] Verificare in curs de: %s @ %s (pid %d), de %v\n",
				ld.User, ld.Computer, ld.PID, time.Since(ld.Started).Round(time.Second))
			fmt.Println("   => In v1 doar avertizam; utilizatorul poate continua oricum.")
		} else {
			fmt.Printf("[VECHI] Lock abandonat de %s @ %s (de %v) - se poate ignora.\n",
				ld.User, ld.Computer, time.Since(ld.Started).Round(time.Second))
		}
		done()

	case "hold":
		sec := atoiDef(arg(args, 2, "30"), 30)
		header("LOCK HOLD", monthDir)
		if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
			fail("Nu pot crea folderul de raport", err)
			return
		}
		// arata daca exista deja un lock al altcuiva.
		if ld, fresh, err := readLock(lockPath); err == nil && fresh {
			fmt.Printf("[AVERTISMENT] Exista deja un lock: %s @ %s (de %v)\n",
				ld.User, ld.Computer, time.Since(ld.Started).Round(time.Second))
		}
		if err := writeLock(lockPath); err != nil {
			fail("Nu pot scrie lock-ul", err)
			return
		}
		okf("Lock pus de %s. Il tin %d secunde.", hostUser(), sec)
		fmt.Println("   => Acum ruleaza pe alt PC: decont-diag lock status <aceeasi luna>")
		for i := sec; i > 0; i-- {
			fmt.Printf("\r   ... mai sunt %3d sec  ", i)
			time.Sleep(time.Second)
		}
		_ = os.Remove(lockPath)
		fmt.Printf("\r   Lock eliberat.            \n")
		done()

	default:
		fail("Subcomanda lock necunoscuta (status | hold)", nil)
	}
}

func readLock(path string) (lockData, bool, error) {
	var ld lockData
	b, err := os.ReadFile(path)
	if err != nil {
		return ld, false, err
	}
	if err := json.Unmarshal(b, &ld); err != nil {
		return ld, false, err
	}
	fresh := time.Since(ld.Started) < lockStale
	return ld, fresh, nil
}

func writeLock(path string) error {
	ld := lockData{
		User:     os.Getenv("USERNAME"),
		Computer: os.Getenv("COMPUTERNAME"),
		PID:      os.Getpid(),
		Started:  time.Now(),
	}
	b, _ := json.MarshalIndent(ld, "", "  ")
	return os.WriteFile(path, b, 0o644)
}

// ---------------------------------------------------------------------------
// open - deschide un fisier chiar daca e deschis in Excel
// ---------------------------------------------------------------------------

func cmdOpen(path string) {
	if path == "" {
		fail("Lipseste fisierul. Ex: decont-diag open \"...\\nota.xlsx\"", nil)
		return
	}
	header("OPEN", path)

	// exista lock ~$ de Excel langa fisier?
	dir, name := filepath.Split(path)
	lock := filepath.Join(dir, "~$"+name)
	if _, err := os.Stat(lock); err == nil {
		fmt.Println("[INFO] Fisierul pare DESCHIS in Excel acum (exista ~$ lock).")
	} else {
		fmt.Println("[INFO] Niciun lock ~$ (probabil nu e deschis in Excel).")
	}

	t0 := time.Now()
	n, err := readOne(path)
	if err != nil {
		fail("Nu am putut citi fisierul", err)
		fmt.Println("   => Daca e deschis in Excel si tot esueaza, conteaza pt modelul de deploy.")
		return
	}
	okf("Citit cu succes: %d randuri in %v (chiar daca e deschis in Excel, citirea merge)",
		n, time.Since(t0).Round(time.Millisecond))
	done()
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// listXlsx gaseste recursiv toate .xlsx sub un folder de luna,
// sarind peste folderul de rapoarte si fisierele temporare ~$.
func listXlsx(root string) []string {
	var out []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.EqualFold(d.Name(), reportFolder) {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, "~$") {
			return nil
		}
		if strings.EqualFold(filepath.Ext(name), ".xlsx") {
			out = append(out, p)
		}
		return nil
	})
	return out
}

func hostUser() string {
	u := os.Getenv("USERNAME")
	c := os.Getenv("COMPUTERNAME")
	return strings.ReplaceAll(u+"_"+c, " ", "_")
}

func human(b int64) string {
	const u = 1024
	if b < u {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(u), 0
	for n := b / u; n >= u; n /= u {
		div *= u
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGT"[exp])
}

func arg(a []string, i int, def string) string {
	if i < len(a) && strings.TrimSpace(a[i]) != "" {
		return a[i]
	}
	return def
}

func atoiDef(s string, def int) int {
	if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return v
	}
	return def
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func header(cmd, target string) {
	fmt.Printf("=== decont-diag %s ===\n", cmd)
	fmt.Printf("User: %s   PC: %s\n", os.Getenv("USERNAME"), os.Getenv("COMPUTERNAME"))
	fmt.Printf("Tinta: %s\n\n", target)
}

func okf(format string, a ...any)   { fmt.Printf("[OK] "+format+"\n", a...) }
func done()                         { fmt.Println("\n=== Gata. ==="); pause() }
func fail(msg string, err error) {
	if err != nil {
		fmt.Printf("[EROARE] %s: %v\n", msg, err)
	} else {
		fmt.Printf("[EROARE] %s\n", msg)
	}
	pause()
}

func pause() {
	fmt.Print("\nApasa Enter pentru a inchide...")
	fmt.Scanln()
}
