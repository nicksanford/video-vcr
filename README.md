## Video VCR

Records rtsp video to a sqlite3 database.

Useful for debugging

### Build:
```bash
make
```

### Record:
```
./bin/linux-aarch64/vcr record rtsp://... recording.db
```

### Replay:
```
./bin/linux-aarch64/vcr replay recording.db
```
