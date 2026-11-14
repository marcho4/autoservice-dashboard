package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/marcho4/autoservice-dashboard/backend/internal/bot"
)

type Config struct {
	HTTPPort        int
	DatabaseURL     string
	JWTSecret       string
	JWTTTL          time.Duration
	TelegramAPIURL  string
	ShutdownTimeout time.Duration
	LogLevel        slog.Level

	BotAPIKey string
	BotRunner bool
	AppURL    string

	LLMBaseURL string
	LLMAPIKey  string
	LLMModel   string
	LLMTimeout time.Duration

	Runner bot.RunnerConfig

	ProbeTargets  map[string]string
	ProbeInterval time.Duration
	ProbeTimeout  time.Duration
}

func LoadDB() (Config, error) {
	var errs []error
	c := Config{
		DatabaseURL: required("DATABASE_URL", &errs),
		LogLevel:    logLevel(&errs),
	}
	return c, errors.Join(errs...)
}

func Load() (Config, error) {
	var errs []error
	c := Config{
		HTTPPort:        intVar("HTTP_PORT", 8080, &errs),
		DatabaseURL:     required("DATABASE_URL", &errs),
		JWTSecret:       required("JWT_SECRET", &errs),
		JWTTTL:          durationVar("JWT_TTL", 720*time.Hour, &errs),
		TelegramAPIURL:  stringVar("TELEGRAM_API_URL", "https://api.telegram.org"),
		ShutdownTimeout: durationVar("SHUTDOWN_TIMEOUT", 15*time.Second, &errs),
		LogLevel:        logLevel(&errs),
		BotAPIKey:       os.Getenv("BOT_API_KEY"),
		BotRunner:       boolVar("BOT_RUNNER", true, &errs),
		AppURL:          os.Getenv("APP_URL"),
	}
	if c.BotAPIKey != "" && len(c.BotAPIKey) < 16 {
		errs = append(errs, errors.New("BOT_API_KEY must be at least 16 characters"))
	}
	loadBot(&c, &errs)
	return c, errors.Join(errs...)
}

func LoadBot() (Config, error) {
	var errs []error
	c := Config{
		DatabaseURL:    required("DATABASE_URL", &errs),
		TelegramAPIURL: stringVar("TELEGRAM_API_URL", "https://api.telegram.org"),
		LogLevel:       logLevel(&errs),
		BotRunner:      true,
		AppURL:         os.Getenv("APP_URL"),
	}
	loadBot(&c, &errs)
	return c, errors.Join(errs...)
}

func loadBot(c *Config, errs *[]error) {
	c.LLMBaseURL = stringVar("LLM_BASE_URL", "https://api.openai.com/v1")
	c.LLMAPIKey = os.Getenv("LLM_API_KEY")
	c.LLMModel = os.Getenv("LLM_MODEL")
	c.LLMTimeout = durationVar("LLM_TIMEOUT", 20*time.Second, errs)
	if u, err := url.Parse(c.LLMBaseURL); err != nil || u.Host == "" {
		*errs = append(*errs, errors.New("LLM_BASE_URL must be an absolute URL like https://api.openai.com/v1"))
	}
}

func LoadProber() (Config, error) {
	var errs []error
	c := Config{
		HTTPPort:        intVar("HTTP_PORT", 8081, &errs),
		ShutdownTimeout: durationVar("SHUTDOWN_TIMEOUT", 15*time.Second, &errs),
		LogLevel:        logLevel(&errs),
		ProbeTargets:    targetsVar("PROBER_TARGETS", &errs),
		ProbeInterval:   durationVar("PROBER_INTERVAL", 15*time.Second, &errs),
		ProbeTimeout:    durationVar("PROBER_TIMEOUT", 5*time.Second, &errs),
	}
	return c, errors.Join(errs...)
}

func stringVar(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func required(name string, errs *[]error) string {
	v := os.Getenv(name)
	if v == "" {
		*errs = append(*errs, fmt.Errorf("%s is required", name))
	}
	return v
}

func intVar(name string, def int, errs *[]error) int {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s must be an integer", name))
	}
	return n
}

func boolVar(name string, def bool, errs *[]error) bool {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s must be true or false", name))
	}
	return b
}

func durationVar(name string, def time.Duration, errs *[]error) time.Duration {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s must be a duration like 24h", name))
	}
	return d
}

func targetsVar(name string, errs *[]error) map[string]string {
	targets := map[string]string{}
	for item := range strings.SplitSeq(required(name, errs), ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		service, target, ok := strings.Cut(item, "=")
		u, err := url.Parse(target)
		if !ok || service == "" || err != nil || u.Host == "" {
			*errs = append(*errs, fmt.Errorf("%s: invalid target %q, expected <service>=<url>", name, item))
			continue
		}
		targets[service] = target
	}
	return targets
}

func logLevel(errs *[]error) slog.Level {
	var l slog.Level
	if err := l.UnmarshalText([]byte(stringVar("LOG_LEVEL", "info"))); err != nil {
		*errs = append(*errs, fmt.Errorf("LOG_LEVEL: %w", err))
	}
	return l
}
