package config

import (
	"fmt"
	"os"
)

type Config struct {
	GeminiAPIKey string
	TavilyAPIKey string
	ExaAPIKey    string
	MaxSearches  int
	MaxWorkers   int
	PromptVersion string
	PromptNote    string
	BlockedDomains []string
	MinScore       float64
}

func Load() *Config {
	cfg := &Config{
		GeminiAPIKey: os.Getenv("GEMINI_API_KEY"),
		TavilyAPIKey: os.Getenv("TAVILY_API_KEY"),
		ExaAPIKey:    os.Getenv("EXA_API_KEY"),
		MaxSearches:  6,
		MaxWorkers:   3,
		PromptVersion: "v3",
		PromptNote:    "Tavily 및 Exa, Scholar 검색 도구 통합",
		MinScore:      0.8,
		BlockedDomains: []string{
            "blog.naver.com",
			"tistory.com",
			"velog.io",
			"reddit.com",
			"stackoverflow.com",
            "medium.com",
        },
	}

	if cfg.GeminiAPIKey == "" {
		fmt.Println("❌ GEMINI_API_KEY 환경변수를 설정해주세요")
		os.Exit(1)
	}
	if cfg.TavilyAPIKey == "" {
		fmt.Println("❌ TAVILY_API_KEY 환경변수를 설정해주세요")
		os.Exit(1)
	}
	if cfg.ExaAPIKey == "" {
		fmt.Println("❌ EXA_API_KEY 환경변수를 설정해주세요")
		os.Exit(1)
	}

	return cfg
}