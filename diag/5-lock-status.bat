@echo off
REM Arata daca cineva are o verificare in curs pe luna asta.
set "LUNA=06.IUNIE"
"%~dp0decont-diag.exe" lock status "\\192.168.2.121\public\Departamente\Contabilitate\aNote Cantar\Note de cantar\2026\%LUNA%"
