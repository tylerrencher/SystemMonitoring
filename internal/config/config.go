package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type IoTawattDevice struct {
	Name string
	Host string
}

type Config struct {
	DatabaseURL string

	IoTawattDevices      []IoTawattDevice
	IoTawattPollInterval time.Duration

	SolarMQTTHost string
	SolarMQTTPort int

	AmbientAPIKey       string
	AmbientAppKey       string
	AmbientMACAddress   string
	AmbientPollInterval time.Duration

	APIListen                 string
	DevCORSOrigin             string
	BatteryCapacityKWh        float64
	SolarActiveThresholdW     float64
	GeneratorActiveThresholdW float64
	TopConsumersWindow        time.Duration

	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
}

func Load() (*Config, error) {
	// Load .env if present. Existing environment variables take precedence,
	// so production deployments using real env vars are unaffected.
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		fmt.Printf("config: .env: %v\n", err)
	}

	cfg := &Config{
		DatabaseURL:               getEnv("DB_URL", "postgres://monitor:monitor@localhost:5432/systemmonitoring"),
		SolarMQTTHost:             getEnv("SOLAR_MQTT_HOST", "192.168.1.56"),
		AmbientAPIKey:             os.Getenv("AMBIENT_API_KEY"),
		AmbientAppKey:             os.Getenv("AMBIENT_APP_KEY"),
		AmbientMACAddress:         os.Getenv("AMBIENT_MAC_ADDRESS"),
		APIListen:                 getEnv("API_LISTEN", ":8080"),
		DevCORSOrigin:             os.Getenv("DEV_CORS_ORIGIN"),
		BatteryCapacityKWh:        parseFloat(getEnv("BATTERY_CAPACITY_KWH", "135")),
		SolarActiveThresholdW:     parseFloat(getEnv("SOLAR_ACTIVE_THRESHOLD_W", "50")),
		GeneratorActiveThresholdW: parseFloat(getEnv("GENERATOR_ACTIVE_THRESHOLD_W", "100")),

		SMTPHost:     getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPUsername: os.Getenv("SMTP_USERNAME"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:     getEnv("SMTP_FROM", os.Getenv("SMTP_USERNAME")),
	}

	if port := os.Getenv("SMTP_PORT"); port != "" {
		fmt.Sscanf(port, "%d", &cfg.SMTPPort)
	} else {
		cfg.SMTPPort = 587
	}

	if port := os.Getenv("SOLAR_MQTT_PORT"); port != "" {
		fmt.Sscanf(port, "%d", &cfg.SolarMQTTPort)
	} else {
		cfg.SolarMQTTPort = 1883
	}

	var err error
	cfg.IoTawattPollInterval, err = parseDuration(getEnv("IOTAWATT_POLL_INTERVAL", "10s"))
	if err != nil {
		return nil, fmt.Errorf("IOTAWATT_POLL_INTERVAL: %w", err)
	}
	cfg.AmbientPollInterval, err = parseDuration(getEnv("AMBIENT_POLL_INTERVAL", "5m"))
	if err != nil {
		return nil, fmt.Errorf("AMBIENT_POLL_INTERVAL: %w", err)
	}
	cfg.TopConsumersWindow, err = parseDuration(getEnv("TOP_CONSUMERS_WINDOW", "30s"))
	if err != nil {
		return nil, fmt.Errorf("TOP_CONSUMERS_WINDOW: %w", err)
	}

	cfg.IoTawattDevices, err = parseDevices(getEnv("IOTAWATT_DEVICES",
		"pwrmone1=pwrmone1.local,pwrmona1=pwrmona1.local,pwrmona2=pwrmona2.local"))
	if err != nil {
		return nil, fmt.Errorf("IOTAWATT_DEVICES: %w", err)
	}

	return cfg, nil
}

func parseDevices(raw string) ([]IoTawattDevice, error) {
	var devices []IoTawattDevice
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid device %q, expected name=host", pair)
		}
		devices = append(devices, IoTawattDevice{Name: parts[0], Host: parts[1]})
	}
	return devices, nil
}

func parseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
