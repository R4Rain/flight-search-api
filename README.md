# Flight Search & Aggregation System

Go HTTP API that aggregates flight data from 4 airline providers, normalizes inconsistent response formats, and returns unified search results with filtering, sorting, caching, and best-value ranking.

---

## Features Implemented

### Core Functionalities

| Feature | Status | Details |
|---|---|---|
| Aggregate from multiple providers | ✅ | 4 providers (Garuda, Lion Air, Batik Air, AirAsia) queried in parallel |
| Normalize different data formats | ✅ | Each provider's unique JSON → unified Flight struct |
| Search by origin, destination, date | ✅ | IATA codes, YYYY-MM-DD format |
| Filter results | ✅ | Price range, stops, airlines, departure/arrival time, duration |
| Sort results | ✅ | Price, duration, departure, arrival, best value |
| Price comparison across providers | ✅ | All prices normalized to IDR, sortable |
| Handle data inconsistencies | ✅ | Timezone differences, missing fields, incorrect stop counts |
| Validate flight data | ✅ | Arrival must be after departure, invalid flights skipped |

### Bonus Features

| Feature | Status | Details |
|---|---|---|
| Best-value scoring algorithm | ✅ | Weighted formula: 60% price + 30% duration + 10% stops |
| Round-trip searches | ✅ | Separate outbound/return arrays, parallel leg search |
| Multi-city searches | ✅ | 2–6 segments, chronological date validation, per-segment results |
| Timezone handling (WIB, WITA, WIT) | ✅ | IANA timezone parsing for Lion Air, offset parsing for Batik Air |
| Rate limiting | ✅ | Per-IP token bucket with idle visitor eviction |
| Retry with exponential backoff | ✅ | AirAsia: 3 retries, 100ms/200ms/400ms backoff |
| IDR currency formatting | ✅ | Displayed as `Rp 1.250.000` with dot separators |
| Parallel provider queries with timeout | ✅ | Goroutines with `context.WithTimeout`, partial results on failure |

---

## Solution Design

1. Client sends search request.

2. The service validates request parameters.

3. The Aggregator dispatches requests to multiple providers

4. Provider responses are normalized into a common format.

5. Results are cached for future requests.

6. Filtering & Sorting

7. A ranking algorithm determines the best-value results.

8. The API returns a JSON response to the client.


```
internal/
├── provider/       # Fetches + normalizes each airline's unique JSON format
├── aggregator/     # Parallel provider orchestration with timeout + multi-leg support
├── filter/         # Filtering, sorting, best-value scoring
├── cache/          # In-memory TTL cache with background cleanup
├── handler/        # HTTP routing, request validation, response formatting
├── middleware/     # Per-IP rate limiting (token bucket)
└── config/         # Environment variable loader with defaults
```

Each concern (data fetching, normalization, filtering, caching) is independently testable and replaceable. Adding a new provider means implementing one interface, no other package changes.

---

## Tech Stack

- **Language:** Go 1.22+
- **Framework:** [Gin](https://github.com/gin-gonic/gin) (HTTP router)
- **Libraries:** `golang.org/x/time/rate` (rate limiting), `log/slog` (structured logging)
- **Mock data:** Embedded via `//go:embed` (zero runtime file dependencies)

---

## Setup & Run Instructions

```bash
# 1. Install dependencies
go mod download

# 2. Verify it compiles
go build ./...

# 3. Run the server (default: port 8080)
go run ./cmd/server/

# 4. Verify
curl http://localhost:8080/health
# → {"status": "ok"}
```

### Configuration (env vars)

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Server port |
| `CACHE_TTL` | `5m` | Cache time-to-live |
| `PROVIDER_TIMEOUT` | `2s` | Max wait per provider (clamped 100ms–10s) |
| `RATE_LIMIT` | `10` | Requests/sec per IP |
| `RATE_BURST` | `20` | Burst capacity |
| `MAX_RETRIES` | `3` | Retry attempts for flaky providers |

```bash
# Custom config example
PORT=3000 CACHE_TTL=10m PROVIDER_TIMEOUT=3s go run ./cmd/server/
```

---

## API Documentation

### `GET /health`, Health check

### `POST /api/v1/flights/search`, Search flights

#### Request Fields

| Field | Type | Required | Notes |
|---|---|---|---|
| `origin` | string | ✅ | 3-letter code |
| `destination` | string | ✅ | 3-letter code |
| `departureDate` | string | ✅ | `YYYY-MM-DD` |
| `passengers` | int | ✅ | 1–9 |
| `cabinClass` | string | ✅ | `economy` / `business` / `first` / `premium_economy` |
| `tripType` | string | ❌ | `one_way` (default) / `round_trip` / `multi_city` |
| `returnDate` | string | ❌* | Required for `round_trip` |
| `segments` | array | ❌* | Required for `multi_city` (each: `{origin, destination, departureDate}`) |
| `filters` | object | ❌ | `minPrice`, `maxPrice`, `maxStops`, `maxDuration`, `airlines[]`, `departureAfter/Before`, `arrivalAfter/Before` (times as `HH:MM`) |
| `sort` | object | ❌ | `by`: `price`/`duration`/`departure`/`arrival`/`best_value`, `order`: `asc`/`desc` |

#### Example Requests

**One-way search with filters:**
```bash
curl -X POST http://localhost:8080/api/v1/flights/search \
  -H "Content-Type: application/json" \
  -d '{
    "origin": "CGK",
    "destination": "DPS",
    "departureDate": "2025-12-15",
    "passengers": 1,
    "cabinClass": "economy",
    "filters": { "maxPrice": 1000000, "maxStops": 0 },
    "sort": { "by": "best_value", "order": "asc" }
  }'
```

**Round-trip:**
```bash
curl -X POST http://localhost:8080/api/v1/flights/search \
  -H "Content-Type: application/json" \
  -d '{
    "tripType": "round_trip",
    "origin": "CGK", "destination": "DPS",
    "departureDate": "2025-12-15", "returnDate": "2025-12-20",
    "passengers": 1, "cabinClass": "economy"
  }'
```

**Multi-city:**
```bash
curl -X POST http://localhost:8080/api/v1/flights/search \
  -H "Content-Type: application/json" \
  -d '{
    "tripType": "multi_city",
    "passengers": 1, "cabinClass": "economy",
    "segments": [
      {"origin": "CGK", "destination": "DPS", "departureDate": "2025-12-15"},
      {"origin": "DPS", "destination": "SUB", "departureDate": "2025-12-18"},
      {"origin": "SUB", "destination": "CGK", "departureDate": "2025-12-20"}
    ]
  }'
```

#### Response Formats

**One-way** → `{ search_criteria, metadata, flights[] }`

**Round-trip** → `{ search_criteria, metadata, outbound_flights[], return_flights[] }`

**Multi-city** → `{ search_criteria, metadata, segments: { "0": {route, flights[]}, "1": ... } }`

Each `Flight` object contains:

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier (e.g., `GA401_GarudaIndonesia`) |
| `provider` | string | Source provider name |
| `airline` | object | `{ name, code }`, airline name and IATA code |
| `flight_number` | string | Flight number (e.g., `GA401`) |
| `departure` | object | `{ airport, city, datetime }`, IATA code, city name, datetime with timezone offset (e.g., `2025-12-15T08:00:00+07:00`) |
| `arrival` | object | `{ airport, city, datetime }`, same structure as departure |
| `duration` | object | `{ total_minutes, formatted }`, e.g., `{ 100, "1h 40m" }` |
| `stops` | int | Number of stops (0 = direct) |
| `price` | object | `{ amount, currency, display }`, e.g., `{ 850000, "IDR", "Rp 850.000" }` |
| `available_seats` | int | Remaining seats |
| `cabin_class` | string | `economy` / `business` / `first` / `premium_economy` |
| `aircraft` | string? | Aircraft type (nullable) |
| `amenities` | string[] | List of amenities |
| `baggage` | object | `{ carry_on, checked }`, baggage allowance descriptions |
| `score` | float? | Best-value score (only present when sorted by `best_value`) |

#### Error Responses

| Status | When |
|---|---|
| `400` | Validation error (invalid IATA code, bad date, missing required fields) |
| `429` | Rate limit exceeded |

---

## Design Decisions

### Request Validation

- Validates **before** hitting any provider (fail-fast approach)
- Checks performed:
  - `tripType` must be `one_way`, `round_trip`, or `multi_city`
  - `origin` and `destination` must be 3-letter IATA codes and cannot be the same
  - `departureDate` must be valid `YYYY-MM-DD`
  - `passengers` between 1–9, `cabinClass` from allowed set
  - Round-trip: `returnDate` required and must be after `departureDate`
  - Multi-city: 2–6 segments, each with valid route, dates must be chronological (each segment's date ≥ previous)
  - Filters: price ranges non-negative, `minPrice` ≤ `maxPrice`, time fields in `HH:MM` format

### Data Normalization

Each provider returns a completely different JSON structure. All are normalized into a single `Flight` struct.

**Garuda Indonesia**
- Reads flights from `status` + `flights[]`
- Price taken from `price.amount` with `price.currency`
- Times are already in standard format with timezone offset (`+07:00`), parsed directly
- Handles multi-segment flights: if a flight has more than 1 segment, uses the first segment's departure and the last segment's arrival, and infers `stops` from segment count (e.g., GA315 declares `stops: 0` but has 2 segments, so we correct it to `stops: 1`)
- Duration calculated from timestamp difference for multi-segment, or from `duration_minutes` for single-segment
- City names provided in the response, falls back to IATA lookup if missing

**Lion Air**
- Reads flights from `success` + `data.available_flights[]`
- Price taken from `pricing.total`
- Times come **without timezone offset**, paired with IANA timezone names (e.g., `Asia/Jakarta`, `Asia/Makassar`), parsed using `time.ParseInLocation()` to attach the correct timezone
- Stops determined by `is_direct` flag and `stop_count`
- Duration calculated from timestamp difference (not from `flight_time`)
- Cabin class extracted from `pricing.fare_type`, lowercased
- Amenities built from boolean flags (`wifi_available`, `meals_included`)

**Batik Air**
- Reads flights from `code: 200` + `results[]`
- Price uses `fare.totalPrice` (base + taxes combined)
- Times use a non-standard timezone offset format without colon (`+0700` instead of `+07:00`), handled by a custom parser that tries standard format first, then falls back to the no-colon format
- Duration calculated from timestamp difference, with `travelTime` string (e.g., `"1h 45m"`) as fallback parsed via regex
- Fare class codes converted to readable names (`Y` → economy, `C` → business, `F` → first)
- City names not provided, resolved via IATA→city lookup map

**AirAsia**
- Reads flights from `status: "ok"` + `flights[]`
- Price taken directly from `price_idr` (already in IDR)
- Times are in standard format with timezone offset, parsed directly
- Duration provided as float hours (`duration_hours: 1.67`) but we calculate from timestamp difference instead for accuracy
- Stops determined by `direct_flight` flag and `stops[]` array length
- City names not provided, resolved via IATA→city lookup map
- Has a 10% simulated failure rate, handled by retry with exponential backoff (100ms → 200ms → 400ms)

### Aggregation

- All 4 providers are queried **in parallel** using goroutines
- A shared `context.WithTimeout` (default 2s) acts as a deadline for all providers
- Each provider runs independently: if one is slow or fails, it does not block others
- Results are collected via a shared slice (index-based, no channels needed since we `wg.Wait()`)
- **Partial results:** if a provider fails (e.g., AirAsia's 10% error rate), the response still returns results from successful providers. Failed providers are listed in `metadata.failed_providers`
- **Multi-leg parallel:** for round-trip and multi-city, each leg is also searched in parallel via `SearchLegs()`. A 3-segment multi-city search fires off 3 independent searches simultaneously
- Results are only cached when at least 1 provider succeeds

### Filtering & Sorting

- Filtering and sorting happen **after** cache retrieval (cache stores raw unfiltered results)
- Each flight is checked against all active filter criteria. A flight must pass **all** filters to be included
- Supported filters: price range, max stops, max duration, airlines (name or IATA code), departure/arrival time windows
- Time-based filters compare only the `HH:MM` portion of the datetime
- Sorting uses `sort.SliceStable` to preserve original ordering for equal elements
- Sort options: `price`, `duration`, `departure`, `arrival`, `best_value`

### Caching Strategy

- **What's cached:** Raw aggregated provider results (before filtering/sorting)
- **Why pre-filter:** One cache entry serves many different filter/sort combinations without re-fetching from providers
- **Cache key:** SHA-256 hash of `origin|destination|date|passengers|cabinClass`
- **TTL:** 5 minutes (configurable via `CACHE_TTL` env var)
- **Cleanup:** Background goroutine evicts expired entries every 60 seconds
- **Multi-leg:** Each leg is cached independently. A round-trip CGK→DPS reuses the cached outbound if a one-way was searched earlier

### Best-Value Scoring

```
score = 0.6 × normalizedPrice + 0.3 × normalizedDuration + 0.1 × normalizedStops
```

- `normalized(x) = (x - min) / (max - min)`, scales each value to [0, 1] range
- Lower score = better value
- Scoring runs in 2 passes: first pass finds min/max across all flights, second pass computes each flight's score
- **Why 60/30/10:** Price is the primary decision factor for domestic Indonesian flights. Duration matters for multi-stop routes. Stops is a tiebreaker (most CGK→DPS flights are direct)
- When all values are equal (e.g., all flights have 0 stops), that dimension contributes 0 to the score

### Rate Limiter

- Per-IP token bucket algorithm using `golang.org/x/time/rate`
- Each unique IP gets its own limiter (default: 10 requests/sec, burst of 20)
- IP obtained via Gin's `c.ClientIP()` which checks `X-Forwarded-For` → `X-Real-IP` → `RemoteAddr`
- **Memory management:** A background goroutine runs every 5 minutes and evicts visitors idle for more than 15 minutes, preventing unbounded memory growth
- Uses `sync.RWMutex` with double-check locking for concurrent-safe access

---

## API Performance

### Complexity

| Operation | Complexity | Notes |
|---|---|---|
| Provider fetch | O(P) parallel | P = 4 providers, all concurrent |
| Aggregation/merge | O(N) | N = total flights across providers |
| Filtering | O(N × F) | F = number of active filter criteria |
| Sorting | O(N log N) | `sort.SliceStable` |
| Best-value scoring | O(N) | Two passes: one for min/max, one for scoring |
| Cache lookup | O(1) | Hash map with SHA-256 key |

### Measured Latency

Tested locally:

| Scenario | Cold (no cache) | Cached |
|---|---|---|
| One-way (13 flights) | ~350ms | ~70ms |
| Round-trip (26 flights) | ~220ms | ~65ms |
| Multi-city 3-seg (39 flights) | ~270ms | ~70ms |

- Cached responses are **~5× faster**, only filter/sort overhead remains
- Round-trip cold is faster than one-way when outbound leg was already cached

---

## Testing

```bash
# Run all 39 tests
go test ./... -v

# Run with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Test Coverage

| Package | What's covered |
|---|---|
| `provider` | Each adapter's normalization (time parsing, price extraction, segment handling, city lookup) |
| `filter` | All filter types, all sort modes, best-value scoring, combined filter+sort |
| `cache` | Set/get, cache miss, TTL expiry, key generation |
| `handler` | Full HTTP flow, one-way, round-trip, multi-city, validation error cases |
| `model` | `FormatDuration`, `FormatIDR` utilities |
