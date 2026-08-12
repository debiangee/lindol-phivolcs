# Webhook Integration

Receive earthquake events as JSON payloads to your own endpoint.

## Configuration

Set the webhook URL in your `.env`:

```env
WEBHOOK_URL=https://your-app.com/api/earthquakes/webhook
```

## Payload Format

### Event: `earthquake.detected`

Sent immediately when a new earthquake is detected from USGS.

```json
{
  "event": "earthquake.detected",
  "earthquake": {
    "id": "us7000abc1",
    "magnitude": 5.2,
    "latitude": 14.50,
    "longitude": 121.00,
    "depth_km": 33,
    "event_time": "2026-08-11T04:38:00Z",
    "location_description": "10 km NW of Manila, Philippines",
    "enriched": false,
    "created_at": "2026-08-11T04:40:00Z",
    "updated_at": "2026-08-11T04:40:00Z"
  },
  "timestamp": "2026-08-11T04:40:01Z"
}
```

### Event: `earthquake.enriched`

Sent after PHIVOLCS data is added (usually 3–5 minutes later).

```json
{
  "event": "earthquake.enriched",
  "earthquake": {
    "id": "us7000abc1",
    "magnitude": 5.2,
    "latitude": 14.50,
    "longitude": 121.00,
    "depth_km": 33,
    "event_time": "2026-08-11T04:38:00Z",
    "location_description": "10 km NW of Manila, Philippines",
    "phivolcs_intensity": "IV",
    "phivolcs_felt_areas": ["Quezon City", "Manila", "Makati"],
    "phivolcs_bulletin_url": "https://earthquake.phivolcs.dost.gov.ph/2026_Earthquake_Information/...",
    "enriched": true,
    "created_at": "2026-08-11T04:40:00Z",
    "updated_at": "2026-08-11T04:45:00Z"
  },
  "timestamp": "2026-08-11T04:45:01Z"
}
```

## Receiving the Webhook

### Node.js / Express Example

```javascript
app.post('/api/earthquakes/webhook', (req, res) => {
  const { event, earthquake } = req.body;

  if (event === 'earthquake.detected') {
    console.log(`New M${earthquake.magnitude} earthquake: ${earthquake.location_description}`);
    // Trigger your app logic here
  }

  if (event === 'earthquake.enriched') {
    console.log(`Enriched: Intensity ${earthquake.phivolcs_intensity}`);
    // Update your UI, send push notifications, etc.
  }

  res.status(200).send('ok');
});
```

### Python / FastAPI Example

```python
@app.post("/api/earthquakes/webhook")
async def handle_webhook(payload: dict):
    event = payload["event"]
    earthquake = payload["earthquake"]

    if event == "earthquake.detected":
        print(f"New M{earthquake['magnitude']}: {earthquake['location_description']}")

    return {"status": "ok"}
```

## Headers

Lindol API sends the following headers with webhook requests:

| Header | Value |
|--------|-------|
| `Content-Type` | `application/json` |
| `User-Agent` | `lindol-api/1.0` |

## Retry Behavior

- Webhook requests have a **10 second timeout**
- If your endpoint returns a 4xx or 5xx status, the event is **not retried** (to avoid duplicate notifications)
- Ensure your endpoint responds within 10 seconds

## Security

To verify the webhook is from Lindol API, you can:

1. Whitelist the IP of your Lindol API server
2. Check the `User-Agent` header
3. (Future) HMAC signature verification will be added
