# Apple Music Downloader Wrapper API

A REST API wrapper for [apple-music-downloader](https://github.com/zhaarey/apple-music-downloader) that provides HTTP endpoints for downloading Apple Music content with real-time status tracking and progress monitoring.

## Prerequisites

- Docker installed on your system
- Apple Music account credentials (configured in `config.yaml`)

## Configuration

Create a `config.yaml` file in your project directory with your Apple Music credentials:

```yaml
# Apple Music configuration
# Add your credentials here
```

## Usage

### Running the Container

**Basic run:**
```bash
docker run -d \
  -p 8080:8080 \
  -v ./downloads:/downloads \
  -v ./config.yaml:/app/config.yaml \
  --name apple-music-api \
  apple-music-api
```

or

```yaml
services:
    apple-music-api:
    build: .
    ports:
        - "8080:8080"
    volumes:
        - ./downloads:/downloads
- ./config.yaml:/app/config.yaml
restart: unless-stopped
```

**With Docker Compose:**

use `compose.yaml` file

First login to Apple Music:

```bash
docker run -it 
  -v ./data:/app/rootfs/data 
  -e args='-L username:password' 
  tikhonp/apple-music-wrapper
```

Then run:

```bash
docker compose up -d
```

### API Endpoints

#### 1. Start a Download

**Endpoint:** `POST /download`

**Request Body:**
```json
{
  "url": "https://music.apple.com/ru/album/children-of-forever/1443732441",
  "format": "alac",
  "song": false,
  "debug": false
}
```

**Parameters:**
- `url` (required): Apple Music URL (album, playlist, or song)
- `format` (optional): Audio format - `"alac"` (default), `"atmos"`, or `"aac"`
- `song` (optional): Set to `true` for single song downloads
- `debug` (optional): Enable debug mode for detailed output

**Example:**
```bash
curl -X POST http://localhost:8080/download \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://music.apple.com/ru/album/children-of-forever/1443732441",
    "format": "alac"
  }'
```

**Response:**
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "started"
}
```

#### 2. Check Job Status

**Endpoint:** `GET /status/{job_id}`

**Example:**
```bash
curl http://localhost:8080/status/550e8400-e29b-41d4-a716-446655440000
```

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "url": "https://music.apple.com/ru/album/children-of-forever/1443732441",
  "status": "running",
  "progress": "Downloading track 3 of 10...",
  "started_at": "2024-12-15T10:30:00Z",
  "logs": [
    "Starting download...",
    "Downloading track 1 of 10...",
    "Downloading track 2 of 10...",
    "Downloading track 3 of 10..."
  ]
}
```

**Status values:**
- `pending`: Job created, waiting to start
- `running`: Download in progress
- `completed`: Download finished successfully
- `failed`: Download failed (check `error` field)

#### 3. List All Jobs

**Endpoint:** `GET /jobs`

**Example:**
```bash
curl http://localhost:8080/jobs
```

**Response:**
```json
{
  "jobs": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "url": "https://music.apple.com/ru/album/children-of-forever/1443732441",
      "status": "completed",
      "started_at": "2024-12-15T10:30:00Z",
      "ended_at": "2024-12-15T10:35:00Z"
    }
  ],
  "count": 1
}
```

#### 4. Health Check

**Endpoint:** `GET /health`

**Example:**
```bash
curl http://localhost:8080/health
```

**Response:**
```json
{
  "status": "healthy"
}
```

## Examples

### Download an Album (ALAC - default)
```bash
curl -X POST http://localhost:8080/download \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://music.apple.com/ru/album/children-of-forever/1443732441"
  }'
```

### Download with Dolby Atmos
```bash
curl -X POST http://localhost:8080/download \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://music.apple.com/us/album/1989-taylors-version-deluxe/1713845538",
    "format": "atmos"
  }'
```

### Download AAC Format
```bash
curl -X POST http://localhost:8080/download \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://music.apple.com/us/album/1989-taylors-version-deluxe/1713845538",
    "format": "aac"
  }'
```

### Download a Single Song
```bash
curl -X POST http://localhost:8080/download \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://music.apple.com/ru/album/bass-folk-song/1443732441?i=1443732453",
    "song": true
  }'
```

### Download a Playlist
```bash
curl -X POST http://localhost:8080/download \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://music.apple.com/us/playlist/taylor-swift-essentials/pl.3950454ced8c45a3b0cc693c2a7db97b"
  }'
```

### Download with Debug Mode
```bash
curl -X POST http://localhost:8080/download \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://music.apple.com/ru/album/miles-smiles/209407331",
    "debug": true
  }'
```

## Monitoring Downloads

You can monitor download progress by polling the status endpoint:

```bash
# Start download and capture job_id
JOB_ID=$(curl -s -X POST http://localhost:8080/download \
  -H "Content-Type: application/json" \
  -d '{"url": "https://music.apple.com/ru/album/children-of-forever/1443732441"}' \
  | jq -r '.job_id')

# Monitor status
watch -n 2 "curl -s http://localhost:8080/status/$JOB_ID | jq"
```

# FINALLY

Completly vibecoded but i know what i m doing :)))))))))
