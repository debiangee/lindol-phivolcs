CREATE TABLE IF NOT EXISTS earthquakes (
    id                   TEXT PRIMARY KEY,
    magnitude            REAL NOT NULL,
    latitude             REAL NOT NULL,
    longitude            REAL NOT NULL,
    depth_km             REAL NOT NULL,
    event_time           TEXT NOT NULL,
    location_description TEXT NOT NULL,
    phivolcs_intensity   TEXT,
    phivolcs_felt_areas  TEXT,
    phivolcs_bulletin_url TEXT,
    enriched             INTEGER NOT NULL DEFAULT 0,
    created_at           TEXT NOT NULL,
    updated_at           TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_earthquakes_event_time ON earthquakes(event_time);
CREATE INDEX IF NOT EXISTS idx_earthquakes_magnitude ON earthquakes(magnitude);
CREATE INDEX IF NOT EXISTS idx_earthquakes_enriched ON earthquakes(enriched);
