# Apple Music Downloader HTTP wrapper

An HTTP API for [apple-music-downloader](https://github.com/zhaarey/apple-music-downloader). Start a download with a POST request, then poll for its status and logs.

## Setup

You need Docker and an Apple Music account.

1. Create `config.yaml` next to `compose.yaml`. The downloader reads its settings from this file, so see the [apple-music-downloader config](https://github.com/zhaarey/apple-music-downloader) for the available options.
2. Log in once. This stores the session in `./data`:

   ```bash
   docker run -it \
     -v ./data:/app/rootfs/data \
     -e args='-L username:password' \
     tikhonp/apple-music-wrapper
   ```

3. Start the services:

   ```bash
   docker compose up -d
   ```

The API listens on port 8080 and saves files to `./downloads`.

> [!TIP]
> To change the user that owns the downloaded files, set `PUID` and `PGID` on the `apple-music-api` service. Both default to `10001`.

## API

### POST /download

Starts a download and returns a job ID.

| Field | Required | Description |
| --- | --- | --- |
| `url` | yes | Apple Music album, playlist, or song URL |
| `format` | no | `alac` (default), `atmos`, or `aac` |
| `song` | no | Has no effect. The wrapper detects a single song from the URL (`/song/` or `?i=`) |
| `debug` | no | Enables debug output |

```bash
curl -X POST http://localhost:8080/download \
  -H "Content-Type: application/json" \
  -d '{"url": "https://music.apple.com/ru/album/children-of-forever/1443732441", "format": "alac"}'
```

```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "started"
}
```

### GET /status/{job_id}

Returns the job state and its logs.

```bash
curl http://localhost:8080/status/550e8400-e29b-41d4-a716-446655440000
```

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "url": "https://music.apple.com/ru/album/children-of-forever/1443732441",
  "status": "running",
  "progress": "Downloading track 3 of 10...",
  "started_at": "2024-12-15T10:30:00Z",
  "logs": [
    "Starting download...",
    "Downloading track 1 of 10..."
  ]
}
```

`status` is one of `pending`, `running`, `completed`, or `failed`. A failed job has an `error` field.

### GET /jobs

Lists all jobs.

```bash
curl http://localhost:8080/jobs
```

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

### GET /health

Returns `{"status": "healthy"}`.

## Example: download and wait

```bash
JOB_ID=$(curl -s -X POST http://localhost:8080/download \
  -H "Content-Type: application/json" \
  -d '{"url": "https://music.apple.com/ru/album/children-of-forever/1443732441"}' \
  | jq -r '.job_id')

watch -n 2 "curl -s http://localhost:8080/status/$JOB_ID | jq"
```
