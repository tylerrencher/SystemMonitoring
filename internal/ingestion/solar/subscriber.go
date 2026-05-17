package solar

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/jackc/pgx/v5/pgxpool"
)

type message struct {
	device string
	metric string
	value  string
	t      time.Time
}

type Subscriber struct {
	host string
	port int
	pool *pgxpool.Pool
	msgs chan message
}

func NewSubscriber(host string, port int, pool *pgxpool.Pool) *Subscriber {
	return &Subscriber{
		host: host,
		port: port,
		pool: pool,
		msgs: make(chan message, 1000),
	}
}

func (s *Subscriber) Run(ctx context.Context) {
	clientID := fmt.Sprintf("systemmonitoring-%d", rand.Int31())
	opts := mqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://%s:%d", s.host, s.port)).
		SetClientID(clientID).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(10 * time.Second).
		SetOnConnectHandler(func(c mqtt.Client) {
			log.Printf("[solar] connected to MQTT broker %s:%d", s.host, s.port)
			c.Subscribe("solar_assistant/#", 0, s.handleMessage)
		}).
		SetConnectionLostHandler(func(_ mqtt.Client, err error) {
			log.Printf("[solar] MQTT connection lost: %v", err)
		})

	client := mqtt.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		log.Printf("[solar] initial connect failed: %v", err)
	}

	go s.writeLoop(ctx)

	<-ctx.Done()
	client.Disconnect(500)
}

func (s *Subscriber) handleMessage(_ mqtt.Client, msg mqtt.Message) {
	// Topic: solar_assistant/{device}/{metric}/state
	parts := strings.Split(msg.Topic(), "/")
	if len(parts) != 4 || parts[0] != "solar_assistant" || parts[3] != "state" {
		return
	}
	s.msgs <- message{
		device: parts[1],
		metric: parts[2],
		value:  string(msg.Payload()),
		t:      time.Now(),
	}
}

func (s *Subscriber) writeLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-s.msgs:
			if err := s.write(ctx, m); err != nil {
				log.Printf("[solar] write error: %v", err)
			}
		}
	}
}

func (s *Subscriber) write(ctx context.Context, m message) error {
	class := Classify(m.device, m.metric)

	switch class {
	case ClassTimeSeries:
		return s.writeTimeSeries(ctx, m)
	case ClassTotal:
		return s.writeTotal(ctx, m)
	case ClassConfig:
		return s.writeConfigChange(ctx, m)
	}
	return nil
}

func (s *Subscriber) writeTimeSeries(ctx context.Context, m message) error {
	numVal, numErr := strconv.ParseFloat(m.value, 32)
	var valueNumeric *float32
	var valueText *string
	if numErr == nil {
		f := float32(numVal)
		valueNumeric = &f
	} else {
		valueText = &m.value
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO solar_readings (time, inverter, metric, value_numeric, value_text)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT DO NOTHING
	`, m.t, m.device, m.metric, valueNumeric, valueText)
	return err
}

func (s *Subscriber) writeTotal(ctx context.Context, m message) error {
	numVal, numErr := strconv.ParseFloat(m.value, 32)
	var valueNumeric *float32
	var valueText *string
	if numErr == nil {
		f := float32(numVal)
		valueNumeric = &f
	} else {
		valueText = &m.value
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO solar_totals (time, metric, value_numeric, value_text)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT DO NOTHING
	`, m.t, m.metric, valueNumeric, valueText)
	return err
}

func (s *Subscriber) writeConfigChange(ctx context.Context, m message) error {
	// Read last known value for this inverter+key
	var lastValue string
	err := s.pool.QueryRow(ctx, `
		SELECT new_value FROM solar_config_changes
		WHERE inverter = $1 AND key = $2
		ORDER BY time DESC LIMIT 1
	`, m.device, m.metric).Scan(&lastValue)

	// If no prior value or value changed, write a new row
	if err != nil || lastValue != m.value {
		var oldValue *string
		if err == nil {
			oldValue = &lastValue
		}
		_, err = s.pool.Exec(ctx, `
			INSERT INTO solar_config_changes (time, inverter, key, old_value, new_value)
			VALUES ($1, $2, $3, $4, $5)
		`, m.t, m.device, m.metric, oldValue, m.value)
		return err
	}
	return nil
}
