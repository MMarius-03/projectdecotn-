package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// reportColumns - antetul raportului, in ordinea ceruta.
var reportColumns = []string{
	"Data", "Rand", "Sofer", "Masina", "Furnizor / Client", "Produs",
	"Cant plecat", "Cant livrat", "Retur", "Dif",
}

// buildReportName creeaza numele unic al raportului.
func buildReportName(luna string) string {
	user := sanitize(getenvDefault("USERNAME", "user"))
	pc := sanitize(getenvDefault("COMPUTERNAME", "pc"))
	ts := time.Now().Format("20060102_150405")
	return fmt.Sprintf("Diferente_%s_%s_%s_%s.xlsx", sanitize(luna), user, pc, ts)
}

// writeReport scrie raportul formatat si returneaza calea completa.
// thresholdFrac = pragul (fractie, ex 0.01 = 1%) peste care DIF e colorat ROSU.
// incompleteFiles = notele excluse (CANT LIVRAT necompletat), salvate intr-o foaie separata.
func writeReport(monthDir, reportFolder, luna string, diffs []DiffRow, thresholdFrac, minKg float64, incompleteFiles []string) (string, error) {
	reportDir := filepath.Join(monthDir, reportFolder)
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		return "", fmt.Errorf("nu pot crea folderul de raport: %w", err)
	}
	outPath := filepath.Join(reportDir, buildReportName(luna))

	f := excelize.NewFile()
	defer f.Close()
	const sheet = "Diferente"
	idx, _ := f.NewSheet(sheet)
	f.SetActiveSheet(idx)
	_ = f.DeleteSheet("Sheet1")

	// stiluri.
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"305496"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    thinBorder(),
	})
	// ROSU: diferenta reala semnificativa (> prag% SI >= minim kg).
	redStyle, _ := f.NewStyle(&excelize.Style{
		Fill:   excelize.Fill{Type: "pattern", Color: []string{"FFC7CE"}, Pattern: 1},
		Font:   &excelize.Font{Color: "9C0006", Bold: true},
		Border: thinBorder(),
		NumFmt: 4, // 1234.00
	})
	// ROSU PALID: diferenta peste prag% dar sub minimul de kg (mai usoara).
	paleRedStyle, _ := f.NewStyle(&excelize.Style{
		Fill:   excelize.Fill{Type: "pattern", Color: []string{"FCE4E6"}, Pattern: 1},
		Font:   &excelize.Font{Color: "C0656D"},
		Border: thinBorder(),
		NumFmt: 4,
	})
	// GALBEN PALID: transfer - marfa preluata dintr-un loc si dusa in altul
	// (diferenta mare, >= prag).
	yellowStyle, _ := f.NewStyle(&excelize.Style{
		Fill:   excelize.Fill{Type: "pattern", Color: []string{"FFF2CC"}, Pattern: 1},
		Font:   &excelize.Font{Color: "7F6000"},
		Border: thinBorder(),
		NumFmt: 4,
	})
	// PORTOCALIU: rand cu RETUR.
	orangeStyle, _ := f.NewStyle(&excelize.Style{
		Fill:   excelize.Fill{Type: "pattern", Color: []string{"FFC000"}, Pattern: 1},
		Font:   &excelize.Font{Color: "7F4F00", Bold: true},
		Border: thinBorder(),
		NumFmt: 4,
	})
	numStyle, _ := f.NewStyle(&excelize.Style{Border: thinBorder(), NumFmt: 4})
	cellStyle, _ := f.NewStyle(&excelize.Style{
		Border:    thinBorder(),
		Alignment: &excelize.Alignment{Vertical: "center", WrapText: true},
	})

	// antet.
	for c, name := range reportColumns {
		cell, _ := excelize.CoordinatesToCellName(c+1, 1)
		_ = f.SetCellValue(sheet, cell, name)
		_ = f.SetCellStyle(sheet, cell, cell, headerStyle)
	}

	// identifica perechile de transfer (+X / -X) inainte de a colora.
	transfer := markTransfers(diffs)

	// randuri de date.
	for i, d := range diffs {
		r := i + 2
		set := func(col int, v any) {
			cell, _ := excelize.CoordinatesToCellName(col, r)
			_ = f.SetCellValue(sheet, cell, v)
			_ = f.SetCellStyle(sheet, cell, cell, cellStyle)
		}
		setNum := func(col int, v float64, style int) {
			cell, _ := excelize.CoordinatesToCellName(col, r)
			_ = f.SetCellValue(sheet, cell, v)
			_ = f.SetCellStyle(sheet, cell, cell, style)
		}
		set(1, d.Data)
		set(2, d.Rand)
		set(3, d.Sofer)
		set(4, d.Masina)
		set(5, d.Client)
		set(6, d.Produs)
		setNum(7, d.CantPlecat, numStyle)
		setNum(8, d.CantLivrat, numStyle)
		setNum(9, d.Retur, returCellStyle(d, orangeStyle, numStyle))
		setNum(10, d.Dif, difCellStyle(d, transfer[i], thresholdFrac, minKg, redStyle, paleRedStyle, yellowStyle, numStyle))
	}

	// latimi coloane (in ordinea: Data, Rand, Sofer, Masina, Client, Produs,
	// Plecat, Livrat, Retur, Dif).
	widths := []float64{12, 7, 18, 12, 26, 26, 11, 11, 9, 9}
	for c, w := range widths {
		col, _ := excelize.ColumnNumberToName(c + 1)
		_ = f.SetColWidth(sheet, col, col, w)
	}

	// filtre + inghetare antet.
	lastCol, _ := excelize.ColumnNumberToName(len(reportColumns))
	_ = f.AutoFilter(sheet, fmt.Sprintf("A1:%s1", lastCol), []excelize.AutoFilterOptions{})
	_ = f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	_ = f.SetRowHeight(sheet, 1, 30)

	// foaie separata cu notele incomplete (excluse din raport).
	addIncompleteSheet(f, headerStyle, incompleteFiles)

	if err := f.SaveAs(outPath); err != nil {
		return "", fmt.Errorf("nu pot salva raportul (poate e deschis sau fara drepturi): %w", err)
	}
	return outPath, nil
}

// addIncompleteSheet adauga o foaie cu notele incomplete (CANT LIVRAT necompletat).
func addIncompleteSheet(f *excelize.File, headerStyle int, files []string) {
	const sheet = "Note incomplete"
	if _, err := f.NewSheet(sheet); err != nil {
		return
	}
	_ = f.SetCellValue(sheet, "A1", "Note incomplete - CANT LIVRAT necompletat (excluse din raport)")
	_ = f.MergeCell(sheet, "A1", "B1")
	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "7F6000"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"FFF2CC"}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	_ = f.SetCellStyle(sheet, "A1", "B1", titleStyle)
	_ = f.SetRowHeight(sheet, 1, 24)

	_ = f.SetCellValue(sheet, "A3", "Nr.")
	_ = f.SetCellValue(sheet, "B3", "Fisier")
	_ = f.SetCellStyle(sheet, "A3", "B3", headerStyle)

	for i, name := range files {
		r := i + 4
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", r), i+1)
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", r), name)
	}
	_ = f.SetColWidth(sheet, "A", "A", 6)
	_ = f.SetColWidth(sheet, "B", "B", 50)
	if len(files) > 0 {
		_ = f.AutoFilter(sheet, "A3:B3", []excelize.AutoFilterOptions{})
	}
}

// returCellStyle: portocaliu daca exista RETUR pe rand.
func returCellStyle(d DiffRow, orange, normal int) int {
	if math.Abs(d.Retur) > eps {
		return orange
	}
	return normal
}

// markTransfers identifica perechile de transfer: marfa preluata de la un client
// (DIF pozitiv, ex +7) si dusa la altul (DIF negativ, ex -7), in aceeasi nota
// (acelasi sofer/masina), acelasi produs si aceeasi cantitate.
// Returneaza un slice paralel cu diffs: true daca randul face parte dintr-o pereche.
func markTransfers(diffs []DiffRow) []bool {
	transfer := make([]bool, len(diffs))
	byFile := map[string][]int{}
	for i := range diffs {
		byFile[diffs[i].Fisier] = append(byFile[diffs[i].Fisier], i)
	}
	for _, idxs := range byFile {
		usedNeg := map[int]bool{}
		for _, i := range idxs {
			if diffs[i].Dif <= eps { // doar pozitivele cauta perechea
				continue
			}
			prod := normHeader(diffs[i].Produs)
			for _, j := range idxs {
				if usedNeg[j] || diffs[j].Dif >= -eps { // doar negativele, nefolosite
					continue
				}
				if normHeader(diffs[j].Produs) != prod {
					continue
				}
				// +X si -X se anuleaza (aceeasi cantitate, semn opus).
				if math.Abs(diffs[i].Dif+diffs[j].Dif) <= eps {
					transfer[i] = true
					transfer[j] = true
					usedNeg[j] = true
					break
				}
			}
		}
	}
	return transfer
}

// difCellStyle decide culoarea celulei DIF:
//   - GALBEN   : transfer - randul face parte dintr-o pereche +X / -X (vezi markTransfers).
//   - ROSU     : diferenta peste prag% din plecat SI |DIF| >= minKg.
//   - ROSU PALID: diferenta peste prag% dar sub minKg (mai usoara).
//   - normal   : DIF = 0, transfer, sau diferenta sub prag% (neglijabila).
func difCellStyle(d DiffRow, isTransfer bool, thresholdFrac, minKg float64, red, paleRed, yellow, normal int) int {
	if math.Abs(d.Dif) <= eps {
		return normal
	}
	if isTransfer {
		return yellow
	}
	if d.CantPlecat > 0 && math.Abs(d.Dif) > thresholdFrac*d.CantPlecat {
		if math.Abs(d.Dif) >= minKg {
			return red
		}
		return paleRed
	}
	return normal
}

func thinBorder() []excelize.Border {
	return []excelize.Border{
		{Type: "left", Color: "D0D0D0", Style: 1},
		{Type: "right", Color: "D0D0D0", Style: 1},
		{Type: "top", Color: "D0D0D0", Style: 1},
		{Type: "bottom", Color: "D0D0D0", Style: 1},
	}
}

// sanitize elimina caracterele nepermise in nume de fisier.
func sanitize(s string) string {
	s = strings.TrimSpace(s)
	repl := strings.NewReplacer(
		"\\", "_", "/", "_", ":", "_", "*", "_", "?", "_",
		"\"", "_", "<", "_", ">", "_", "|", "_", " ", "_",
	)
	return repl.Replace(s)
}

func getenvDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
