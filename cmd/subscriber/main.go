package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"database/sql"

	"github.com/censys/scan-takehome/pkg/scanning"
	_ "modernc.org/sqlite"

	"cloud.google.com/go/pubsub"
)

func main() {
	ctx := context.Background()

	projectId := flag.String("project", "test-project", "GCP Project ID")
	subscriptionId := flag.String("subscription", "scan-sub", "GCP PubSub Subscription ID")

	client, err := pubsub.NewClient(ctx, *projectId)
	if err != nil {
		log.Fatalf("pubsub.NewClient: %v", err)
	}
	defer client.Close()

	sub := client.Subscription(*subscriptionId)

	log.Printf("listening for messages on subscription %v...", *subscriptionId)

	db, err := initDB()
	if err != nil {
		log.Fatalf("initDB: %v", err)
	}
	defer db.Close()

	err = sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
		aff, err := processMessage(ctx, db, m.Data)
		if err != nil {
			log.Printf("process error (nack): %v", err)
			m.Nack()
			return
		}

		// quick outcome log for easier manual verification: ignored/upserted
		if aff == 0 {
			log.Printf("scans: ignored older message")
		} else {
			log.Printf("scans: upserted/inserted message (rows=%d)", aff)
		}

		m.Ack()
	})

	if err != nil {
		log.Fatalf("sub.Receive: %v", err)
	}
}

// will return unmarshalled response or "" on any error
// corrupted messages won't be retried (empty string is stored and msg is acked)
func normalize(scan scanning.Scan) string {
	data, err := json.Marshal(scan.Data)
	if err != nil {
		return ""
	}

	switch scan.DataVersion {
	case scanning.V1:
		var v1 scanning.V1Data
		if err := json.Unmarshal(data, &v1); err == nil && len(v1.ResponseBytesUtf8) > 0 {
			return string(v1.ResponseBytesUtf8)
		}
	case scanning.V2:
		var v2 scanning.V2Data
		if err := json.Unmarshal(data, &v2); err == nil && v2.ResponseStr != "" {
			return v2.ResponseStr
		}

	}

	return ""
}

func initDB() (*sql.DB, error) {
	if err := os.MkdirAll("/data", 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", "/data/subscriber.db")
	if err != nil {
		return nil, err
	}

	// ensure the DB is reachable before proceeding
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	log.Println("SQLite opened at /data/subscriber.db")

	db.SetMaxOpenConns(1)

	// mounting might cause lags - wait up to 3s before failing
	if _, err := db.Exec(`PRAGMA busy_timeout=3000;`); err != nil {
		log.Printf("set busy_timeout: %v", err)
	}

	// enable WAL for faster writes and concurrent reads
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		log.Printf("set journal_mode=WAL: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS scans (
		ip          TEXT    NOT NULL,
		port        INTEGER NOT NULL,
		service     TEXT    NOT NULL,
		scanned_at  INTEGER NOT NULL,
		response    TEXT,
		received_at INTEGER NOT NULL,
		PRIMARY KEY (ip, port, service)
		);
	`)
	if err != nil {
		db.Close()
		return nil, err
	}
	log.Println("SQLite table ready (scans).")

	return db, nil
}

func processMessage(ctx context.Context, db *sql.DB, data []byte) (int64, error) {
	var scan scanning.Scan
	if err := json.Unmarshal(data, &scan); err != nil {
		return 0, err
	}

	response := normalize(scan)

	res, err := db.Exec(`
		INSERT INTO scans (ip, port, service, scanned_at, response, received_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(ip, port, service) DO UPDATE SET
			scanned_at  = excluded.scanned_at,
			response    = excluded.response,
			received_at = excluded.received_at
		WHERE excluded.scanned_at > scans.scanned_at;
	`, scan.Ip, scan.Port, scan.Service, scan.Timestamp, response, time.Now().Unix())
	if err != nil {
		return 0, err
	}

	aff, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}

	return aff, nil
}
