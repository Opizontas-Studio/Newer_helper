#!/bin/bash

# Load environment variables from .env file if it exists
if [ -f .env ]; then
  set -a # automatically export all variables
  source .env
  set +a # stop automatically exporting
fi

# Check if BOT_TOKEN and APP_ID are set
if [ -z "$BOT_TOKEN" ]; then
  echo "Error: BOT_TOKEN is not set. Please create a .env file with your BOT_TOKEN."
  exit 1
fi

if [ -z "$APP_ID" ]; then
  echo "Error: APP_ID is not set. Please create a .env file with your APP_ID."
  exit 1
fi

# A sample Guild ID to test against. You can change this to any guild your bot is in.
GUILD_ID="1291925535324110879"
URL="https://discord.com/api/v10/applications/$APP_ID/guilds/$GUILD_ID/commands"

echo "Checking command rate limit status for Guild ID: $GUILD_ID..."
echo "----------------------------------------------------"

# Perform the curl request and store the full response (headers + body)
response=$(curl -s -i -H "Authorization: Bot $BOT_TOKEN" "$URL")

# Extract the HTTP status code from the first line of the response
status_code=$(echo "$response" | head -n 1 | awk '{print $2}')

echo "HTTP Status: $status_code"

# Check if we are rate limited (HTTP 429)
if [ "$status_code" -eq 429 ]; then
  echo "Status: RATE LIMITED"
  # Extract the 'retry-after' header value
  retry_after=$(echo "$response" | grep -i "retry-after:" | awk '{print $2}' | tr -d '\r')
  if [ -n "$retry_after" ]; then
    echo "You need to wait for approximately $retry_after seconds before retrying."
  else
    echo "Could not determine the exact wait time from the response."
  fi
else
  echo "Status: OK"
  # Extract other rate limit headers
  remaining=$(echo "$response" | grep -i "x-ratelimit-remaining:" | awk '{print $2}' | tr -d '\r')
  reset_after=$(echo "$response" | grep -i "x-ratelimit-reset-after:" | awk '{print $2}' | tr -d '\r')
  
  echo "Requests remaining in this window: $remaining"
  echo "Rate limit resets in: $reset_after seconds"
fi

echo "----------------------------------------------------"