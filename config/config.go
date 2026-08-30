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
	PromptVersion string
	PromptNote    string
	BlockedDomains []string
	MinScore       float64
}

func Load() *Config {
	cfg := &Config{
		GeminiAPIKey: os.Getenv("GEMINI_API_KEY"),
		TavilyAPIKey: os.Getenv("TAVILY_API_KEY"),
		MaxSearches:  6,
		MaxWorkers:   3,
		PromptVersion: "v2",
		PromptNote:    "검색 의도 다양화 및 중복 쿼리 제거",
		MinScore:      0.7,
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

	return cfg
}