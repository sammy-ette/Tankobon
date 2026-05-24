# Tankobon
> It's Sonarr for manga (like 99%!)

Tankobon is a self-hosted manga library manager.
It searches Prowlarr indexers for manga releases to download based on your library, sends them
to qBittorrent, and imports them in an organized file structure built for Kavita.

## Features

- Track manga series sourced from MangaBaka
- Monitor series for new volume/chapter releases
- Automatically detect and import downloads from qBittorrent
- Search Prowlarr indexers for monitored series on a schedule
- Manual import UI with per-file volume/chapter mapping
- Import history and activity queue

## How It Works

**Import cycle** runs every 30 seconds (fast cycle) or 5 minutes (slow cycle). Speed is picked
based on if there are active torrents. It checks qBittorrent for completed torrents in the configured 
category, parses archive filenames to extract volume/chapter numbers, and matches them to tracked series.
Files are hard-linked into the library path (source and destination must be on the same filesystem).

**Search cycle** runs every 12 hours.
It checks MangaBaka for updated volume counts, then searches Prowlarr for any missing volumes of monitored
series. Matching torrents are added to qBittorrent automatically.

Torrents that can't be automatically matched (unknown series, ambiguous files) will appear in the
**Activity** page under "Needs Attention" for manual review.
The manual import UI lets you assign each file to a specific volume or chapter before importing.

## Install
### Docker
Simple command to pull and run Tankobon:

```
docker run -p 5505:5505 -it ghcr.io/sammy-ette/tankobon:main
```

Or build it locally once you've cloned the repository:

```
docker build -t tankobon .
docker run -p 5505:5505 -it tankobon
```

### Compose

```yaml
services:
  tankobon:
    image: ghcr.io/sammy-ette/tankobon:main
    container_name: tankobon
    restart: unless-stopped
    ports:
      - "5505:5505"
    volumes:
      - ./data:/app/data
      - /path/to/manga/library:/library
    environment:
      - TANKOBON_JWT_SECRET=change-me
```

It doesn't have to be set specifically to `/library`,
you can set it to what you want as long as it matches the Library Path.
This is where Tankobon will hard-link imported files.

### Source
Tankobon is portable as long as you keep the frontend files (in `dist/`) next to the executable.
```
git clone https://github.com/sammy-ette/Tankobon
cd Tankobon
gleam run -m lustre/dev build --minify
go build
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `TANKOBON_JWT_SECRET` | `change-me-in-production` | Secret used to sign JWT tokens. CHANGE THIS OR YOUR INSTANCE IS INSECURE. |

## Configuration

After first run, create an account and go to **Settings** to configure the integrations:

| Field | Description |
|---|---|
| Prowlarr URL | Base URL of your Prowlarr instance (e.g. `http://prowlarr:9696`) |
| Prowlarr API Token | Found in Prowlarr → Settings → General |
| qBittorrent URL | Base URL of your qBittorrent instance (e.g. `http://qbittorrent:8080`) |
| qBittorrent Username | qBittorrent web UI username |
| qBittorrent Password | qBittorrent web UI password |
| qBittorrent Category | The category Tankobon should watch for new downloads |
| Library Path | Absolute path inside the container where imported files are hard-linked |

# License
Tankobon is AGPL 3.0 licensed.