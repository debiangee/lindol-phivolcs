# API Usage Examples

Base URL: `http://localhost:3000` (or your deployed server)

## Health Check

```bash
curl http://localhost:3000/api/health
```

```json
{
  "status": "ok",
  "service": "lindol-api",
  "timestamp": "2026-08-11T07:17:37Z",
  "uptime": "2h30m15s",
  "sources": {
    "usgs": { "name": "USGS", "status": "up", "last_success": "2026-08-11T07:16:00Z", "error_count": 0 },
    "phivolcs": { "name": "PHIVOLCS", "status": "up", "last_success": "2026-08-11T07:15:30Z", "error_count": 0 }
  }
}
```

## List Recent Earthquakes

```bash
# Default: last 20 earthquakes
curl http://localhost:3000/api/earthquakes

# Filter by magnitude
curl "http://localhost:3000/api/earthquakes?minMagnitude=4.0"

# Filter by date range
curl "http://localhost:3000/api/earthquakes?startDate=2026-08-01&endDate=2026-08-11"

# Pagination
curl "http://localhost:3000/api/earthquakes?limit=10&offset=20"

# Combined
curl "http://localhost:3000/api/earthquakes?minMagnitude=3.0&limit=5&startDate=2026-08-10"
```

Response:
```json
{
  "data": [
    {
      "id": "us7000abc1",
      "magnitude": 5.2,
      "latitude": 14.50,
      "longitude": 121.00,
      "depth_km": 33,
      "event_time": "2026-08-11T04:38:00Z",
      "location_description": "10 km NW of Manila, Philippines",
      "phivolcs_intensity": "IV",
      "phivolcs_felt_areas": ["Quezon City", "Manila", "Makati"],
      "phivolcs_bulletin_url": "https://earthquake.phivolcs.dost.gov.ph/...",
      "enriched": true,
      "created_at": "2026-08-11T04:40:00Z",
      "updated_at": "2026-08-11T04:45:00Z"
    }
  ],
  "total": 47,
  "limit": 20,
  "offset": 0
}
```

## Get Latest Earthquake

```bash
curl http://localhost:3000/api/earthquakes/latest
```

Returns a single earthquake object (most recent by event_time).

## Get Earthquake by ID

```bash
curl http://localhost:3000/api/earthquakes/us7000abc1
```

Returns 404 if not found:
```json
{ "error": "Earthquake not found" }
```

## Using from JavaScript (Frontend)

```javascript
// Fetch latest earthquake
const response = await fetch('https://your-server.com/api/earthquakes/latest');
const earthquake = await response.json();

console.log(`M${earthquake.magnitude} - ${earthquake.location_description}`);
```

## Using from Python

```python
import requests

# Get recent significant quakes
resp = requests.get('https://your-server.com/api/earthquakes', params={
    'minMagnitude': 4.0,
    'limit': 10
})

data = resp.json()
for eq in data['data']:
    print(f"M{eq['magnitude']} - {eq['location_description']}")
```

## Using from Flutter/Dart

```dart
final response = await http.get(
  Uri.parse('https://your-server.com/api/earthquakes/latest'),
);

if (response.statusCode == 200) {
  final earthquake = jsonDecode(response.body);
  print('M${earthquake['magnitude']} - ${earthquake['location_description']}');
}
```

## Query Parameters Reference

| Parameter | Type | Description | Example |
|-----------|------|-------------|---------|
| `minMagnitude` | float | Minimum magnitude | `3.0` |
| `maxMagnitude` | float | Maximum magnitude | `7.0` |
| `startDate` | string | Start date (ISO 8601 or YYYY-MM-DD) | `2026-08-01` |
| `endDate` | string | End date (ISO 8601 or YYYY-MM-DD) | `2026-08-11` |
| `limit` | int | Results per page (max 100) | `10` |
| `offset` | int | Pagination offset | `20` |
