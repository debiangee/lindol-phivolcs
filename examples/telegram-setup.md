# Setting Up Telegram Notifications

Get earthquake alerts delivered to your Telegram.

## Step 1: Create a Telegram Bot

1. Open Telegram and search for **@BotFather**
2. Send `/newbot`
3. Follow the prompts — choose a name and username
4. BotFather will give you a **bot token** like: `123456789:ABCdefGHIjklMNOpqrsTUVwxyz`
5. Save this token

## Step 2: Get Your Chat ID

1. Send any message to your new bot
2. Visit: `https://api.telegram.org/bot<YOUR_TOKEN>/getUpdates`
3. Look for `"chat": {"id": 123456789}` in the response
4. That number is your **chat ID**

### For Group Chats

1. Add your bot to the group
2. Send a message in the group
3. Check `/getUpdates` — the chat ID for groups is negative (e.g., `-1001234567890`)

## Step 3: Configure Lindol API

Add to your `.env` file:

```env
TELEGRAM_BOT_TOKEN=replace_with_your_bot_token
TELEGRAM_CHAT_ID=replace_with_your_chat_id
```

## Step 4: Restart the Service

```bash
# If running directly
./lindol-api

# If using Docker
docker compose restart
```

You'll see in the logs:
```
level=INFO msg="Telegram notifications enabled"
```

## What You'll Receive

**When a new earthquake is detected:**

```
🚨 Earthquake Detected

Magnitude: 5.2
Location: 10 km NW of Manila, Philippines
Coordinates: 14.50°N, 121.00°E
Depth: 33 km
Time: 11 Aug 2026 - 12:38 PM PHT
Source: USGS
```

**After PHIVOLCS enrichment (3–5 min later):**

```
📋 Update: M5.2 Earthquake

Intensity: IV (Moderately Strong)
Felt in: Quezon City, Manila, Makati
View PHIVOLCS Bulletin
```

## Tips

- Set `MIN_MAGNITUDE=4.0` if you only want to be notified about significant quakes
- You can add the bot to a group chat for team/family alerts
- The bot only sends messages — it doesn't read or respond to commands
