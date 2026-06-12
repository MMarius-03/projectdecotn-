@echo off
REM Viteza de citire peste retea: secvential vs 4 workeri paraleli.
REM Schimba luna in linia LUNA daca vrei alta luna.
set "LUNA=06.IUNIE"
"%~dp0decont-diag.exe" bench "\\192.168.2.121\public\Departamente\Contabilitate\aNote Cantar\Note de cantar\2026\%LUNA%" 4
