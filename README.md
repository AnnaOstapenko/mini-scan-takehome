# Mini Scan Take-Home

This project simulates a simple message processing using Google Pub/Sub (via the emulator):

- **scanner** publishes synthetic scan results to a Pub/Sub topic
- **subscriber** consumes scans and stores the _latest_ scan per `(ip, port, service)` in SQLite
    - The subscriber writes data to ./data/subscriber.db
---

## Requirements & Design Considerations
This service was designed as a quick 4-hour solution for a take-home assignment. The goal was to deliver a correct, maintainable, and horizontally scalable solution while intentionally avoiding out-of-scope enhancements.

Functional Requirements:
- **at-least-once semantics** Every message should be processed at least once. The subscriber acknowledges messages only after they are successfully normalized and written to storage.
- **horizontal scalability** Running n copies of the app will not impact logic or behaviour. This is achieved by:
    - using UPSERT logic keyed by (ip, port, service)
    - ensuring idempotency (older scans never overwrite newer ones)
    - processing each message independently with no shared in-memory state
- **storage-agnostic design** Although sqlite was chosen as a simple and quick solution, the design doesn't rely on anything sqlite-specific so swapping the storage layer won’t affect behavior.

Constraints and Considerations:

Implementation was done under a strict time limit and focused only on the specified requirements, intentionally omitting changes outside the assignment scope. This includes following the existing semantics for consistency and readability (keeping the same Go version as the scanner, avoiding upgrades or optimizations to the provided logic, following the same code structure etc). Non-essential concerns such as authentication, monitoring, configuration management, graceful shutdown, etc were intentionally skipped. Code was also broken down into small helper functions purely for readability, not as an architectural necessity.

## Running locally

Requirements:

- Docker + Docker Compose  
- (Optional) sqlite to inspect stored data

Start the project:

```bash
docker compose up --build
```

Stop with Ctrl+C.

## Testing

### Automated tests

Run:
```bash 
go test ./cmd/subscriber 
```

Covers:
- V1 normalization
- V2 normalization
- corrupted/malformed data handling

### Manual tests

#### Smoke test  
Start the system and inspect stored scans:
1. docker compose up --build 
2. in a separate terminal: sqlite3 ./data/subscriber.db  
3. SELECT COUNT(*) FROM scans;
4. SELECT ip, port, service, response FROM scans LIMIT 5;

You should see rows with response like _"service response: [some number]."_

#### Latest-only behavior  

1. Pick an existing row from scans table.
2. Increase its timestamp manually (+1 day etc).
3. Temporary change cmd/scanner/main.go to publish messages with _the same_ ip, port and service (as scan in step #2)
4. Start the project - scans should be ignored 
