package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"io/fs"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
	"github.com/tylerrencher/systemmonitoring/internal/alerts"
	"github.com/tylerrencher/systemmonitoring/internal/api"
	"github.com/tylerrencher/systemmonitoring/internal/config"
	appdb "github.com/tylerrencher/systemmonitoring/internal/db"
	"github.com/tylerrencher/systemmonitoring/internal/ingestion/iotawatt"
	"github.com/tylerrencher/systemmonitoring/internal/ingestion/solar"
	"github.com/tylerrencher/systemmonitoring/internal/ingestion/weather"
	uipkg "github.com/tylerrencher/systemmonitoring/ui"
)

func uiFS() fs.FS {
	return uipkg.FS()
}

func main() {
	root := &cobra.Command{
		Use:   "monitor",
		Short: "Home energy monitoring ingestion service",
	}

	root.AddCommand(serveCmd(), migrateCmd(), backfillCmd(), userAddCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start ingestion service",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("config: %w", err)
			}

			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			pool, err := appdb.Connect(ctx, cfg.DatabaseURL)
			if err != nil {
				return fmt.Errorf("db: %w", err)
			}
			defer pool.Close()

			alertState := alerts.NewState()

			var wg sync.WaitGroup

			wg.Add(1)
			go func() {
				defer wg.Done()
				alerts.NewEngine(pool, cfg, alertState).Run(ctx)
			}()

			for _, dev := range cfg.IoTawattDevices {
				dev := dev
				wg.Add(1)
				go func() {
					defer wg.Done()
					s := iotawatt.NewScraper(dev.Name, dev.Host, pool, cfg.IoTawattPollInterval)
					s.Run(ctx)
				}()
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				sub := solar.NewSubscriber(cfg.SolarMQTTHost, cfg.SolarMQTTPort, pool)
				sub.Run(ctx)
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				p := weather.NewPoller(
					cfg.AmbientAPIKey, cfg.AmbientAppKey, cfg.AmbientMACAddress,
					pool, cfg.AmbientPollInterval,
				)
				p.Run(ctx)
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				srv := api.NewServer(cfg, pool).WithStatic(uiFS()).WithAlertState(alertState)
				if err := srv.Serve(ctx); err != nil {
					log.Printf("[api] error: %v", err)
				}
			}()

			log.Printf("monitor serving — press Ctrl+C to stop")
			wg.Wait()
			return nil
		},
	}
}

func migrateCmd() *cobra.Command {
	var migrationsDir string
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			ctx := context.Background()
			pool, err := appdb.Connect(ctx, cfg.DatabaseURL)
			if err != nil {
				return err
			}
			defer pool.Close()
			return appdb.Migrate(ctx, pool, migrationsDir)
		},
	}
	cmd.Flags().StringVar(&migrationsDir, "migrations-dir", "migrations", "path to SQL migration files")
	return cmd
}

func backfillCmd() *cobra.Command {
	var (
		source string
		from   string
		to     string
	)

	cmd := &cobra.Command{
		Use:   "backfill",
		Short: "Backfill historical data for one or all sources",
		Long: `Backfill fetches historical data from the specified source and writes it to the database.
Omit --from to use the stored watermark as the start time.

Sources: pwrmone1, pwrmona1, pwrmona2, solar_assistant, ambient_weather, all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			ctx := context.Background()
			pool, err := appdb.Connect(ctx, cfg.DatabaseURL)
			if err != nil {
				return err
			}
			defer pool.Close()

			fromTime, toTime, err := parseTimeRange(source, from, to)
			if err != nil {
				return err
			}

			log.Printf("backfill %s: %s → %s", source, fromTime.Format(time.DateOnly), toTime.Format(time.DateOnly))

			switch source {
			case "ambient_weather":
				p := weather.NewPoller(cfg.AmbientAPIKey, cfg.AmbientAppKey, cfg.AmbientMACAddress, pool, 0)
				return p.Backfill(ctx, fromTime, toTime)
			case "all":
				for _, dev := range cfg.IoTawattDevices {
					s := iotawatt.NewScraper(dev.Name, dev.Host, pool, 0)
					if err := s.Backfill(ctx, fromTime, toTime); err != nil {
						log.Printf("backfill %s error: %v", dev.Name, err)
					}
				}
				p := weather.NewPoller(cfg.AmbientAPIKey, cfg.AmbientAppKey, cfg.AmbientMACAddress, pool, 0)
				return p.Backfill(ctx, fromTime, toTime)
			default:
				for _, dev := range cfg.IoTawattDevices {
					if dev.Name == source {
						s := iotawatt.NewScraper(dev.Name, dev.Host, pool, 0)
						return s.Backfill(ctx, fromTime, toTime)
					}
				}
				return fmt.Errorf("unknown source %q", source)
			}
		},
	}

	cmd.Flags().StringVar(&source, "source", "all", "source to backfill")
	cmd.Flags().StringVar(&from, "from", "", "start date (YYYY-MM-DD); defaults to stored watermark")
	cmd.Flags().StringVar(&to, "to", "", "end date (YYYY-MM-DD); defaults to today")
	return cmd
}

func userAddCmd() *cobra.Command {
	var (
		name     string
		password string
		role     string
		phone    string
		email    string
	)

	cmd := &cobra.Command{
		Use:   "useradd",
		Short: "Create a user account",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" || password == "" {
				return fmt.Errorf("--name and --password are required")
			}
			if role != "admin" && role != "viewer" {
				return fmt.Errorf("--role must be admin or viewer")
			}

			hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				return fmt.Errorf("hash password: %w", err)
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			ctx := context.Background()
			pool, err := appdb.Connect(ctx, cfg.DatabaseURL)
			if err != nil {
				return err
			}
			defer pool.Close()

			var phonePtr, emailPtr *string
			if phone != "" {
				phonePtr = &phone
			}
			if email != "" {
				emailPtr = &email
			}

			var id int
			err = pool.QueryRow(ctx, `
				INSERT INTO users (name, password_hash, role, phone, email)
				VALUES ($1, $2, $3, $4, $5)
				RETURNING id
			`, name, string(hash), role, phonePtr, emailPtr).Scan(&id)
			if err != nil {
				return fmt.Errorf("insert user: %w", err)
			}

			log.Printf("created user %q (id=%d, role=%s)", name, id, role)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "username (required)")
	cmd.Flags().StringVar(&password, "password", "", "password (required)")
	cmd.Flags().StringVar(&role, "role", "viewer", "role: admin or viewer")
	cmd.Flags().StringVar(&phone, "phone", "", "phone number for SMS alerts")
	cmd.Flags().StringVar(&email, "email", "", "email address")
	return cmd
}


func parseTimeRange(source, from, to string) (time.Time, time.Time, error) {
	toTime := time.Now()
	if to != "" {
		t, err := time.ParseInLocation(time.DateOnly, to, time.Local)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("--to: %w", err)
		}
		toTime = t.Add(24 * time.Hour)
	}

	if from != "" {
		fromTime, err := time.ParseInLocation(time.DateOnly, from, time.Local)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("--from: %w", err)
		}
		return fromTime, toTime, nil
	}

	// No --from: use watermark
	fromTime := toTime.Add(-30 * 24 * time.Hour) // fallback: 30 days
	return fromTime, toTime, nil
}
