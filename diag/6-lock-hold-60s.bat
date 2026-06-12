@echo off
REM Test intre 2 PC-uri: aici TII lock-ul 60 sec.
REM Pe celalalt PC ruleaza in acelasi timp 5-lock-status.bat -> trebuie sa arate OCUPAT.
set "LUNA=06.IUNIE"
"%~dp0decont-diag.exe" lock hold "\\192.168.2.121\public\Departamente\Contabilitate\aNote Cantar\Note de cantar\2026\%LUNA%" 60
