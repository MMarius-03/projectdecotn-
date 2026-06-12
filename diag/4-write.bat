@echo off
REM Test contentie: 5 "utilizatori" scriu rapoarte in acelasi timp.
set "LUNA=06.IUNIE"
"%~dp0decont-diag.exe" write "\\192.168.2.121\public\Departamente\Contabilitate\aNote Cantar\Note de cantar\2026\%LUNA%" 5
