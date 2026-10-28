# Verificare diferente deconturi

Aplicatie Windows scrisa in Go pentru verificarea notelor de cantar `.xlsx`.
Aplicatia cauta randurile unde exista diferente intre cantitatea plecata,
cantitatea livrata si retur, apoi genereaza un raport Excel separat.

Nu modifica fisierele sursa.

## Ce face

- scaneaza notele de cantar din folderul unei luni;
- proceseaza doar fisiere `.xlsx` din foldere de zi, de forma `ZZ.LL.AAAA`;
- ignora fisiere temporare Excel (`~$...`) si rapoarte generate anterior;
- detecteaza randuri cu `DIF != 0` sau `RETUR != 0`;
- calculeaza `DIF` daca valoarea nu este salvata in fisier;
- completeaza automat clientul de pe randurile urmatoare cand celula `CLIENT` este goala;
- exclude notele incomplete unde `CANT LIVRAT` este gol pe toate randurile;
- genereaza raport in `Rapoarte_Diferente`;
- deschide o interfata locala in browser, pe `127.0.0.1`.

## Rulare pentru utilizatori

In folderul aplicatiei trebuie sa existe impreuna:

- `verificare-diferente.exe`
- `config.json`
- `Verificare diferente.bat`

Pornire:

1. Dublu-click pe `Verificare diferente.bat`.
2. Lasa fereastra de consola deschisa cat timp folosesti aplicatia.
3. Browserul se deschide automat.
4. Alege luna si apasa `Verificare`.
5. Daca se genereaza raport, apasa `Deschidere fisier generat`.

Pentru instructiuni de lucru mai detaliate, vezi [app/CITESTE-MA.txt](app/CITESTE-MA.txt).

## Configurare

Aplicatia citeste `config.json` de langa executabil.

Exemplu:

```json
{
  "base_path": "C:\\Data\\NoteCantar\\2026",
  "report_folder": "Rapoarte_Diferente",
  "port": 0,
  "workers": 4,
  "dif_red_threshold_percent": 1.0,
  "dif_red_min_kg": 1.0
}
```

Campuri:

- `base_path` - folderul anului care contine lunile;
- `report_folder` - subfolderul unde se salveaza rapoartele;
- `port` - `0` inseamna port liber ales automat;
- `workers` - cate fisiere sunt citite in paralel;
- `dif_red_threshold_percent` - prag procentual pentru colorarea rosie a diferentei;
- `dif_red_min_kg` - prag minim in kg pentru colorarea rosie.

Calea poate fi modificata si din interfata, prin `Modificare cale`.

## Raport generat

Rapoartele sunt salvate in folderul lunii:

```text
<folder_luna>\Rapoarte_Diferente\
```

Numele este unic:

```text
Diferente_<luna>_<utilizator>_<calculator>_<yyyyMMdd_HHmmss>.xlsx
```

Raportul contine foaia `Diferente` si foaia `Note incomplete`.

Colorare:

- rosu - diferenta semnificativa;
- rosu palid - diferenta peste prag procentual, dar sub minimul in kg;
- galben - transfer detectat ca pereche `+X` / `-X`;
- portocaliu - rand cu `RETUR`.

## Build pentru dezvoltare

Cerinte:

- Go instalat;
- Windows pentru rularea executabilului final si deschiderea browserului/fisierelor prin comenzi Windows.

Build aplicatie principala:

```powershell
cd app
go mod download
go build -o verificare-diferente.exe .
```

Rulare din sursa:

```powershell
cd app
go run .
```

Verificare rapida:

```powershell
cd app
go test ./...
```

## Structura proiectului

```text
app/      aplicatia principala: UI web local, scanare Excel, rapoarte
diag/     utilitare de diagnostic pentru acces, lock si performanta
probe/    program mic de proba
```

Fisiere importante:

- [app/main.go](app/main.go) - serverul local si endpointurile HTTP;
- [app/ui.go](app/ui.go) - interfata HTML/CSS/JS;
- [app/scan.go](app/scan.go) - citirea unei note Excel;
- [app/scanmonth.go](app/scanmonth.go) - scanarea unei luni cu workeri;
- [app/report.go](app/report.go) - generarea raportului Excel;
- [app/lock.go](app/lock.go) - avertizare pentru verificari simultane.

## Note

Repo-ul ignora executabilele, rapoartele generate, fisierele Excel locale,
capturile de ecran si fisierele locale ale asistentilor. Codul sursa si
fisierele de configurare exemplu raman versionate.
