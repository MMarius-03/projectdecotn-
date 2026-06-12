package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config este citit din config.json de langa executabil.
type Config struct {
	BasePath              string  `json:"base_path"`
	ReportFolder          string  `json:"report_folder"`
	Port                  int     `json:"port"`
	Workers               int     `json:"workers"`
	DifRedThresholdPct    float64 `json:"dif_red_threshold_percent"` // prag ROSU: DIF peste X% din plecat
	DifRedMinKg           float64 `json:"dif_red_min_kg"`             // prag ROSU: si |DIF| de minim X kg
}

func defaultConfig() Config {
	return Config{
		BasePath:           `\\192.168.2.121\public\Departamente\Contabilitate\aNote Cantar\Note de cantar\2026`,
		ReportFolder:       "Rapoarte_Diferente",
		Port:               0,
		Workers:            4,
		DifRedThresholdPct: 1.0,
		DifRedMinKg:        1.0,
	}
}

// exeDir returneaza folderul executabilului (acolo sta config.json).
func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func configPath() string {
	return filepath.Join(exeDir(), "config.json")
}

// loadConfig citeste config.json; daca lipseste, foloseste valorile implicite
// si scrie un fisier nou ca utilizatorul sa-l poata edita.
func loadConfig() Config {
	cfg := defaultConfig()
	b, err := os.ReadFile(configPath())
	if err != nil {
		_ = saveConfig(cfg)
		return cfg
	}
	_ = json.Unmarshal(b, &cfg)
	if cfg.ReportFolder == "" {
		cfg.ReportFolder = "Rapoarte_Diferente"
	}
	if cfg.Workers < 1 {
		cfg.Workers = 4
	}
	if cfg.DifRedThresholdPct <= 0 {
		cfg.DifRedThresholdPct = 1.0
	}
	if cfg.DifRedMinKg <= 0 {
		cfg.DifRedMinKg = 1.0 // 0 sau lipsa -> implicit 1 kg
	}
	return cfg
}

func saveConfig(cfg Config) error {
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(configPath(), b, 0o644)
}
