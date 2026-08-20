package config

import (
	"fmt"
	"os"
)

type Config struct {
	GeminiAPIKey string
	TavilyAPIKey string
	MaxSearches  int
	MaxWorkers   int
}

func Load() *Config {
	cfg := &Config{
		GeminiAPIKey: os.Getenv("GEMINI_API_KEY"),
		TavilyAPIKey: os.Getenv("TAVILY_API_KEY"),
		MaxSearches:  6,
		MaxWorkers:   3,
	}

	if cfg.GeminiAPIKey == "" {
		fmt.Println("❌ GEMINI_API_KEY 환경변수를 설정해주세요")
		os.Exit(1)
	}
	if cfg.TavilyAPIKey == "" {
		fmt.Println("❌ TAVILY_API_KEY 환경변수를 설정해주세요")
		os.Exit(1)
	}

	return cfg
}