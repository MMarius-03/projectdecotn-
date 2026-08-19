# Verificare diferențe deconturi

**VIF: NU**

Aplicație Windows scrisă în Go pentru verificarea notelor de cântar Excel și
generarea unui raport cu diferențele găsite.

## Ce face

- scanează fișierele `.xlsx` organizate pe luni și zile;
- identifică diferențe și retururi;
- ignoră fișierele temporare sau incomplete;
- generează rapoarte Excel în `Rapoarte_Diferente`;
- oferă o interfață locală în browser.

Aplicația lucrează local cu documente Excel și nu folosește Putty sau VIF.
