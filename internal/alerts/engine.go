package alerts

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tylerrencher/systemmonitoring/internal/config"
)

// Engine evaluates all enabled alert definitions every 60 seconds,
// updates shared State, and sends rollup email notifications.
type Engine struct {
	pool  *pgxpool.Pool
	cfg   *config.Config
	state *State
}

func NewEngine(pool *pgxpool.Pool, cfg *config.Config, state *State) *Engine {
	return &Engine{pool: pool, cfg: cfg, state: state}
}

func (e *Engine) Run(ctx context.Context) {
	log.Printf("[alerts] engine starting")
	e.tick(ctx)

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.tick(ctx)
		}
	}
}

func (e *Engine) tick(ctx context.Context) {
	defs, err := loadAlertDefs(ctx, e.pool)
	if err != nil {
		log.Printf("[alerts] load defs: %v", err)
		return
	}

	var active []ActiveAlert
	var activeKeys []string

	for _, def := range defs {
		triggered, err := evaluate(ctx, e.pool, def)
		if err != nil {
			log.Printf("[alerts] evaluate %s: %v", def.Key, err)
			continue
		}
		if triggered {
			log.Printf("[alerts] active: %s", def.Key)
			active = append(active, ActiveAlert{Key: def.Key, Name: def.Name})
			activeKeys = append(activeKeys, def.Key)
		}
	}

	if active == nil {
		active = []ActiveAlert{}
	}
	e.state.set(active)

	if len(activeKeys) == 0 {
		return
	}
	if e.cfg.SMTPPassword == "" {
		log.Printf("[alerts] skipping email: SMTP_PASSWORD not set")
		return
	}

	recipients, err := loadRecipients(ctx, e.pool, activeKeys)
	if err != nil {
		log.Printf("[alerts] load recipients: %v", err)
		return
	}
	if len(recipients) == 0 {
		return
	}

	// Group alerts by user for rollup
	type userBatch struct {
		email  string
		alerts []namedAlert
	}
	byUser := map[int]*userBatch{}
	defByKey := make(map[string]AlertDef, len(defs))
	for _, d := range defs {
		defByKey[d.Key] = d
	}

	for _, r := range recipients {
		if _, ok := byUser[r.UserID]; !ok {
			byUser[r.UserID] = &userBatch{email: r.Email}
		}
		byUser[r.UserID].alerts = append(byUser[r.UserID].alerts, namedAlert{Key: r.AlertKey, Name: r.AlertName})
	}

	for userID, batch := range byUser {
		if err := sendRollup(e.cfg, batch.email, batch.alerts); err != nil {
			log.Printf("[alerts] send rollup to user %d: %v", userID, err)
			continue
		}
		for _, a := range batch.alerts {
			if def, ok := defByKey[a.Key]; ok {
				if err := setMute(ctx, e.pool, userID, a.Key, def.Cooldown); err != nil {
					log.Printf("[alerts] set mute %s user %d: %v", a.Key, userID, err)
				}
			}
		}
	}
}
