package alerts

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AlertDef is a row from the alerts table.
type AlertDef struct {
	ID         int
	Key        string
	Name       string
	Type       string
	DataSource string
	Params     json.RawMessage
	Cooldown   time.Duration
}

// recipient is a user who should receive a notification for an alert this tick.
type recipient struct {
	UserID    int
	Email     string
	AlertKey  string
	AlertName string
}

func loadAlertDefs(ctx context.Context, pool *pgxpool.Pool) ([]AlertDef, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, key, name, type, data_source, params,
		       EXTRACT(EPOCH FROM cooldown)::bigint AS cooldown_secs
		FROM alerts
		WHERE enabled = true
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var defs []AlertDef
	for rows.Next() {
		var d AlertDef
		var cooldownSecs int64
		var params []byte
		if err := rows.Scan(&d.ID, &d.Key, &d.Name, &d.Type, &d.DataSource, &params, &cooldownSecs); err != nil {
			return nil, err
		}
		d.Params = json.RawMessage(params)
		d.Cooldown = time.Duration(cooldownSecs) * time.Second
		defs = append(defs, d)
	}
	return defs, rows.Err()
}

// loadRecipients returns users subscribed to any of the given active alert keys
// who do not currently have an active mute for that alert.
func loadRecipients(ctx context.Context, pool *pgxpool.Pool, activeKeys []string) ([]recipient, error) {
	if len(activeKeys) == 0 {
		return nil, nil
	}
	rows, err := pool.Query(ctx, `
		SELECT u.id, u.email, ap.alert_key, a.name
		FROM alert_preferences ap
		JOIN users u ON u.id = ap.user_id
		JOIN alerts a ON a.key = ap.alert_key
		WHERE ap.alert_key = ANY($1)
		  AND u.email IS NOT NULL
		  AND NOT EXISTS (
		    SELECT 1 FROM alert_mutes am
		    WHERE am.user_id = u.id
		      AND am.alert_key = ap.alert_key
		      AND am.muted_until > NOW()
		  )
	`, activeKeys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []recipient
	for rows.Next() {
		var r recipient
		if err := rows.Scan(&r.UserID, &r.Email, &r.AlertKey, &r.AlertName); err != nil {
			continue
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// setMute records a cooldown mute for a user/alert after a firing.
// GREATEST ensures a user-set snooze longer than the cooldown is never shortened.
func setMute(ctx context.Context, pool *pgxpool.Pool, userID int, alertKey string, cooldown time.Duration) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO alert_mutes (user_id, alert_key, muted_until)
		VALUES ($1, $2, NOW() + make_interval(secs => $3::float))
		ON CONFLICT (user_id, alert_key) DO UPDATE
		SET muted_until = GREATEST(EXCLUDED.muted_until, alert_mutes.muted_until)
	`, userID, alertKey, cooldown.Seconds())
	return err
}
