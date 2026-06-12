# Plan aplicatie Go pentru verificare diferente Excel

## Rezumat

Aplicatia va fi un executabil Windows `.exe`, scris in Go, pus in folderul SMB unde sunt si fisierele deconturilor.

Folderul SMB nu ruleaza aplicatia singur. Fiecare utilizator porneste executabilul din share, iar aplicatia ruleaza local pe calculatorul lui. Interfata se deschide in browser pe `127.0.0.1`, iar fisierele Excel se citesc si se scriu pe share.

Calea implicita:

```text
\\192.168.2.121\public\Departamente\Contabilitate\aNote Cantar\Note de cantar\2026
```

Aplicatia va lista subfolderele de luna din aceasta cale, va scana fisierele `.xlsx` din luna selectata si va genera raport doar daca gaseste valori diferite de zero in coloanele `DIF` sau `RETUR`.

## Tehnologie si rulare

- Limbaj: Go.
- Interfata: server web local Go, deschis in browser.
- Excel: biblioteca Go `excelize`.
- Livrare: un singur fisier `.exe`, copiat in folderul SMB.
- Configurare: fisier `config.json` langa executabil.
- Fiecare utilizator ruleaza propria instanta locala din acelasi folder partajat.

Setari in `config.json`:

```json
{
  "base_path": "\\\\192.168.2.121\\public\\Departamente\\Contabilitate\\aNote Cantar\\Note de cantar\\2026",
  "report_folder": "Rapoarte_Diferente",
  "port": 0
}
```

`port: 0` inseamna ca aplicatia alege automat un port liber si deschide browserul local. Astfel evitam conflictele daca mai multe instante ruleaza pe acelasi calculator.

## Interfata

Interfata trebuie sa fie foarte simpla si sa contina:

- buton `Verificare`;
- buton sau camp `Modificare cale`;
- selector `Selectare luna`;
- buton `Deschidere fisier generat`.

Comportament:

- la pornire, aplicatia citeste calea din `config.json`;
- selectorul de luna se populeaza cu subfolderele gasite in calea anului;
- utilizatorul selecteaza luna si apasa `Verificare`;
- aplicatia afiseaza progres simplu: cate fisiere verifica si cate diferente a gasit;
- daca exista diferente, genereaza raportul si activeaza butonul `Deschidere fisier generat`;
- daca nu exista diferente, afiseaza mesajul `Nu au fost gasite diferente`.

## Logica Excel

Aplicatia va scana toate fisierele `.xlsx` din folderul lunii selectate.

Se vor ignora:

- fisierele temporare Excel care incep cu `~$`;
- fisierele din subfolderul de rapoarte;
- fisierele generate anterior de aplicatie.

Pentru fiecare fisier:

- se cauta foaia care contine tabelul de livrare;
- se identifica anteturile compatibile cu:
  - `CLIENT`;
  - produs, inclusiv antetul `Column1`;
  - `CANT PLECAT`;
  - `CANT LIVRAT`;
  - `RETUR`;
  - `DIF`;
  - `AVIZ`, daca exista.
- se citesc datele generale ale notei:
  - data;
  - numele soferului;
  - numarul de inmatriculare al masinii.

Regula pentru includerea in raport:

```text
DIF != 0 sau RETUR != 0
```

Daca valoarea `DIF` nu este salvata/calculata in fisier, aplicatia o calculeaza:

```text
CANT PLECAT - CANT LIVRAT - RETUR
```

Daca pe un rand campul `CLIENT` este gol, aplicatia foloseste ultimul client completat de deasupra. Aceasta regula este necesara pentru fisierele unde acelasi client are mai multe produse pe randuri consecutive.

## Raport generat

Raportul se salveaza in folderul lunii selectate, intr-un subfolder dedicat:

```text
<folder_luna>\Rapoarte_Diferente\
```

Numele raportului va fi unic:

```text
Diferente_<luna>_<user>_<computer>_<yyyyMMdd_HHmmss>.xlsx
```

Coloane raport:

- Data;
- Sofer;
- Masina;
- Furnizor / Client;
- Produs;
- Cant plecat;
- Cant livrat;
- Retur;
- Dif;
- Aviz;
- Fisier sursa;
- Foaie;
- Rand.

Raportul trebuie formatat simplu:

- antet bold;
- filtre pe coloane;
- coloane late suficient;
- valorile `RETUR` si `DIF` evidentiate vizual.

## Concurenta in folder SMB

Pentru v1, daca aplicatia detecteaza ca alta verificare pare in curs pentru aceeasi luna, afiseaza doar avertizare.

Utilizatorul poate continua.

Rapoartele nu se suprascriu, deoarece fiecare raport are in nume:

- luna;
- utilizatorul Windows;
- numele calculatorului;
- timestamp.

## Erori tratate

Aplicatia trebuie sa afiseze erori clare pentru:

- calea SMB nu exista sau nu este accesibila;
- utilizatorul nu are drepturi de citire;
- utilizatorul nu are drepturi de scriere in folderul de raport;
- fisier Excel corupt sau blocat;
- nu exista subfoldere de luna;
- nu exista fisiere `.xlsx` in luna selectata.

Un fisier problematic nu trebuie sa opreasca toata verificarea. Aplicatia il marcheaza in sumar ca eroare si continua cu urmatorul fisier.

## Test plan

Test principal cu fisierul de proba:

```text
01.06-GHERMAN-BT39PET pentru teste.xlsx
```

Rezultatul asteptat:

- detecteaza randul `STRIVEX TRADING / CARC MANZAT`;
- `DIF = 1.4`;
- data `01.06.2026`;
- sofer `GHERMAN IONUT`;
- masina `BT-39-PET`.

Alte teste:

- rand cu `CLIENT` gol: se foloseste clientul de deasupra;
- rand cu `RETUR` diferit de zero si `DIF = 0`: intra in raport;
- fisier fara diferente: nu se genereaza raport;
- share indisponibil: aplicatia afiseaza eroare fara crash;
- doua rulari simultane: apare avertizare, dar se pot genera rapoarte separate;
- raportul generat se deschide corect in Excel.

## Presupuneri pentru v1

- Suportam doar fisiere `.xlsx`, nu `.xls`.
- Nu adaugam login sau parola in aplicatie; accesul ramane controlat de Windows/SMB.
- Nu modificam fisierele sursa.
- Aplicatia doar citeste notele si creeaza raport separat.
- Structura lunilor vine din subfolderele existente in folderul `2026`.
- Daca formatul unor fisiere difera mult de exemplul primit, acestea vor fi raportate ca fisiere neprocesate, nu ghicite agresiv.
