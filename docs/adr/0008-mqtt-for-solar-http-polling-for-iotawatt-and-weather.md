# Solar data is ingested via MQTT; IoTawatt and weather are ingested via HTTP polling

The three data sources use different ingestion patterns. Solar Assistant publishes every reading to an MQTT broker as it happens (`solar_assistant/#` topic wildcard). IoTawatt exposes a REST query API with a configurable time range. Ambient Weather provides a polling API with rate limits. The ingestion layer mirrors these native interfaces: a persistent MQTT subscriber for solar, and interval-based HTTP pollers for the other two.

This is not architectural inconsistency — it is the interface each device provides. Solar Assistant's prescribed integration path is MQTT; it has no supported REST query API for historical data. Polling Solar Assistant would mean either scraping an internal endpoint (unsupported, likely to break) or missing readings during downtime with no recovery path. The MQTT subscriber receives every data point the moment it is published and buffers up to 1,000 messages in a channel before writing, giving sub-second data latency. IoTawatt's REST API supports arbitrary time range queries, which also enables the backfill command; its poll interval is configurable (default 10 seconds). The weather API is rate-limited by the provider, making event-driven delivery impractical.

## Considered options

- **Poll all three sources via HTTP** — rejected for solar: Solar Assistant does not expose a reliable REST API for reading current metrics; MQTT is the documented integration path.
- **Route all three through a message queue (NATS, Kafka)** — rejected: over-engineered for a single-machine home system; adds an external dependency with no benefit given the source interfaces already differ.
