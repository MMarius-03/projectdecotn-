package main

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// DiffRow este un rand din raport care contine o diferenta gasita.
type DiffRow struct {
	Data       string  `json:"data"`
	Sofer      string  `json:"sofer"`
	Masina     string  `json:"masina"`
	Client     string  `json:"client"`
	Produs     string  `json:"produs"`
	CantPlecat float64 `json:"cant_plecat"`
	CantLivrat float64 `json:"cant_livrat"`
	Retur      float64 `json:"retur"`
	Dif        float64 `json:"dif"`
	Aviz       string  `json:"aviz"`
	Fisier     string  `json:"fisier"`
	Foaie      string  `json:"foaie"`
	Rand       int     `json:"rand"`
}

// FileResult este rezultatul scanarii unui singur fisier.
type FileResult struct {
	Fisier     string
	Diffs      []DiffRow
	Processed  bool // true daca s-a gasit un tabel de livrare in fisier
	Incomplete bool // true daca livrarea nu a fost completata (CANT LIVRAT gol peste tot)
	Err        error
}

// dayRe accepta numele folderelor de zi: ZZ.LL.AAAA, tolerant la puncte duble
// (ex folderul real cu typo "20..03.2026").
var dayRe = regexp.MustCompile(`^\d{2}\.+\d{2}\.\d{4}$`)

// isDayFolder spune daca un nume de folder este un folder de zi valid.
func isDayFolder(name string) bool {
	return dayRe.MatchString(strings.TrimSpace(name))
}

// toleranta pentru comparatii cu numere zecimale (zgomot de virgula mobila).
const eps = 0.0005

var plateRe = regexp.MustCompile(`(?i)^[A-Z]{1,2}[- ]?\d{2,3}[- ]?[A-Z]{3}$`)

// fnPlateRe extrage numarul de inmatriculare din numele fisierului,
// ex "01.05-CUCIUREANU-BT87TIP.xlsx" sau "06.05 BUNDUC BT 81 TIP.xlsx".
var fnPlateRe = regexp.MustCompile(`(?i)([A-Z]{1,2})\s*-?\s*(\d{2,3})\s*-?\s*([A-Z]{3})`)

// plateFromName scoate numarul de inmatriculare din numele fisierului si il
// formateaza ca BT-87-TIP. Returneaza "" daca nu gaseste un tipar de numar.
func plateFromName(fileName string) string {
	name := fileName
	if i := strings.LastIndex(name, "."); i > 0 {
		name = name[:i]
	}
	m := fnPlateRe.FindStringSubmatch(name)
	if m == nil {
		return ""
	}
	return strings.ToUpper(m[1]) + "-" + m[2] + "-" + strings.ToUpper(m[3])
}

// normHeader normalizeaza un antet: majuscule, fara spatii/linii noi.
func normHeader(s string) string {
	s = strings.ToUpper(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, " ", "")
	return strings.TrimSpace(s)
}

// cellAt returneaza valoarea celulei (r,c) sau "" daca nu exista.
func cellAt(rows [][]string, r, c int) string {
	if r < 0 || r >= len(rows) {
		return ""
	}
	if c < 0 || c >= len(rows[r]) {
		return ""
	}
	return rows[r][c]
}

// parseNum incearca sa converteasca un text in numar. Returneaza (val, ok).
func parseNum(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	// accepta atat "." cat si "," ca separator zecimal.
	s = strings.ReplaceAll(s, ",", ".")
	// elimina separatorii de mii / spatii.
	s = strings.ReplaceAll(s, " ", "")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// round3 rotunjeste la 3 zecimale pentru a elimina zgomotul.
func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}

// headerMap retine indicii coloanelor identificate in tabelul de livrare.
type headerMap struct {
	row    int
	client int
	produs int
	plecat int
	livrat int
	retur  int
	dif    int
	aviz   int
}

// findHeader cauta randul de antet intr-o foaie si mapeaza coloanele.
// Returneaza (headerMap, true) daca a gasit un tabel valid (CLIENT + DIF/PLECAT).
func findHeader(rows [][]string) (headerMap, bool) {
	for r := 0; r < len(rows); r++ {
		hm := headerMap{row: r, client: -1, produs: -1, plecat: -1, livrat: -1, retur: -1, dif: -1, aviz: -1}
		hasClient, hasKey := false, false
		for c, raw := range rows[r] {
			n := normHeader(raw)
			switch {
			case n == "CLIENT":
				hm.client = c
				hasClient = true
			case n == "COLUMN1" || strings.Contains(n, "PRODUS"):
				if hm.produs == -1 {
					hm.produs = c
				}
			case n == "CANTPLECAT":
				hm.plecat = c
				hasKey = true
			case n == "CANTLIVRAT":
				hm.livrat = c
			case n == "RETUR":
				hm.retur = c
			case n == "DIF":
				hm.dif = c
				hasKey = true
			case n == "AVIZ":
				hm.aviz = c
			}
		}
		if hasClient && hasKey {
			// daca nu am gasit coloana de produs, foloseste cea de langa CLIENT.
			if hm.produs == -1 && hm.client >= 0 {
				hm.produs = hm.client + 1
			}
			return hm, true
		}
	}
	return headerMap{}, false
}

// noteInfo retine datele generale ale notei (data, sofer, masina).
type noteInfo struct {
	data   string
	sofer  string
	masina string
}

// extractNoteInfo cauta in zona de sus a foii data, soferul si masina.
func extractNoteInfo(rows [][]string, headerRow int) noteInfo {
	var ni noteInfo
	limit := headerRow
	if limit > len(rows) {
		limit = len(rows)
	}
	for r := 0; r < limit; r++ {
		for c, raw := range rows[r] {
			n := normHeader(raw)
			if ni.data == "" && strings.HasPrefix(n, "DATA") {
				ni.data = firstBelow(rows, r, c, headerRow)
			}
			if ni.sofer == "" && strings.HasPrefix(n, "SOFER") {
				ni.sofer = firstBelow(rows, r, c, headerRow)
			}
		}
	}
	ni.masina = findPlate(rows, headerRow)
	return ni
}

// firstBelow returneaza prima celula ne-goala sub (r,c).
func firstBelow(rows [][]string, r, c, limit int) string {
	for rr := r + 1; rr < limit && rr < len(rows) && rr <= r+4; rr++ {
		v := strings.TrimSpace(cellAt(rows, rr, c))
		if v != "" {
			return v
		}
	}
	return ""
}

// findPlate cauta o celula care arata ca un numar de inmatriculare in zona de sus.
// Prefera varianta cu cratime (BT-39-PET) fata de cea lipita (BT39PET).
func findPlate(rows [][]string, limit int) string {
	if limit > len(rows) {
		limit = len(rows)
	}
	best := ""
	for r := 0; r < limit; r++ {
		for _, raw := range rows[r] {
			v := strings.TrimSpace(raw)
			if plateRe.MatchString(v) {
				if strings.Contains(v, "-") {
					return strings.ToUpper(v)
				}
				if best == "" {
					best = strings.ToUpper(v)
				}
			}
		}
	}
	return best
}

// scanFile scaneaza un fisier .xlsx si returneaza diferentele gasite.
func scanFile(path, fileName string) FileResult {
	res := FileResult{Fisier: fileName}
	f, err := excelize.OpenFile(path)
	if err != nil {
		res.Err = err
		return res
	}
	defer f.Close()

	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		hm, ok := findHeader(rows)
		if !ok {
			continue
		}
		ni := extractNoteInfo(rows, hm.row)
		// Numarul de inmatriculare din celula (I1) este adesea valoarea veche
		// din sablon (toate notele sunt copii ale aceluiasi fisier). Numele
		// fisierului contine numarul real, deci il preferam.
		if p := plateFromName(fileName); p != "" {
			ni.masina = p
		}
		lastClient := ""
		var diffs []DiffRow
		deliveryRows := 0 // linii reale de livrare (cu produs)
		livratSeen := 0   // cate au valoare in CANT LIVRAT

		for r := hm.row + 1; r < len(rows); r++ {
			clientRaw := strings.TrimSpace(cellAt(rows, r, hm.client))
			produs := strings.TrimSpace(cellAt(rows, r, hm.produs))

			// rand fara produs: separator sau antet de grup; nu e linie de livrare.
			if produs == "" {
				if clientRaw != "" {
					lastClient = clientRaw
				}
				continue
			}

			// Blocul de sumar/pivot din josul foii: acolo coloana de produs
			// contine un NUMAR (total), nu un nume de produs. Il sarim.
			if _, isNum := parseNum(produs); isNum {
				continue
			}
			np := normHeader(produs)
			nc := normHeader(clientRaw)
			if np == "VALUES" || nc == "VALUES" ||
				strings.Contains(np, "TOTAL") || strings.Contains(nc, "TOTAL") {
				continue
			}

			// forward-fill client (acelasi client cu mai multe produse pe randuri consecutive).
			if clientRaw != "" {
				lastClient = clientRaw
			}
			client := clientRaw
			if client == "" {
				client = lastClient
			}

			deliveryRows++
			plecat, _ := parseNum(cellAt(rows, r, hm.plecat))
			livrat, livratOK := parseNum(cellAt(rows, r, hm.livrat))
			retur, _ := parseNum(cellAt(rows, r, hm.retur))
			dif, difOK := parseNum(cellAt(rows, r, hm.dif))
			if livratOK {
				livratSeen++
			}
			if !difOK {
				dif = plecat - livrat - retur
			}
			dif = round3(dif)
			retur = round3(retur)

			if math.Abs(dif) > eps || math.Abs(retur) > eps {
				diffs = append(diffs, DiffRow{
					Data:       ni.data,
					Sofer:      ni.sofer,
					Masina:     ni.masina,
					Client:     client,
					Produs:     produs,
					CantPlecat: round3(plecat),
					CantLivrat: round3(livrat),
					Retur:      retur,
					Dif:        dif,
					Aviz:       strings.TrimSpace(cellAt(rows, r, hm.aviz)),
					Fisier:     fileName,
					Foaie:      sheet,
					Rand:       r + 1,
				})
			}
		}
		// am procesat foaia cu tabelul de livrare; oprim cautarea.
		res.Processed = true
		// Nota incompleta: livrarea nu a fost completata pe niciun rand
		// (CANT LIVRAT gol peste tot). Atunci DIF = PLECAT nu e o diferenta
		// reala, ci doar o nota nereconciliata -> o excludem din raport.
		if deliveryRows > 0 && livratSeen == 0 {
			res.Incomplete = true
		} else {
			res.Diffs = diffs
		}
		return res
	}
	return res
}
