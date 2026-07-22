#!/bin/bash

# Configuration
API_URL="http://localhost:8080"
JWT_SECRET="rdkfrheriafheriufherousafhaeo"

echo "=== Lyra Stress Test ==="

echo -e "\n1. Initializing Session 1..."
INIT_RES=$(curl -s -X POST $API_URL/init)
TOKEN1=$(echo $INIT_RES | grep -o '"token":"[^"]*' | cut -d'"' -f4)
if [ -z "$TOKEN1" ]; then
    echo "Failed to get token: $INIT_RES"
    exit 1
fi
echo "Got Token 1"

echo -e "\n2. Pinging with Session 1 to grab lock..."
curl -s -X POST $API_URL/ping -H "Authorization: Bearer $TOKEN1"

echo -e "\n\n3. Sending valid message (Session 1)..."
curl -s -X POST $API_URL/sendMessage \
     -H "Authorization: Bearer $TOKEN1" \
     -H "Content-Type: application/json" \
     -d '{"message": "Hello Lyra, how are you today?"}'
     
echo -e "\n\n4. Sending message exceeding max characters (Session 1)..."
# API_MAX_CHARS_PER_MESSAGE=500
LONG_MSG=$(printf 'A%.0s' {1..550})
curl -s -X POST $API_URL/sendMessage \
     -H "Authorization: Bearer $TOKEN1" \
     -H "Content-Type: application/json" \
     -d "{\"message\": \"$LONG_MSG\"}"

echo -e "\n\n5. Initializing Session 2 (Concurrent User)..."
INIT_RES2=$(curl -s -X POST $API_URL/init)
TOKEN2=$(echo $INIT_RES2 | grep -o '"token":"[^"]*' | cut -d'"' -f4)
echo "Got Token 2"

echo -e "\n6. Attempting to Ping with Session 2 (Should fail because Session 1 holds lock)..."
curl -s -X POST $API_URL/ping -H "Authorization: Bearer $TOKEN2"

echo -e "\n\n7. Disconnecting Session 1..."
curl -s -X POST $API_URL/disconnect -H "Authorization: Bearer $TOKEN1"

echo -e "\n\n8. Attempting to Ping with Session 2 immediately after disconnect (Should hit cooldown)..."
curl -s -X POST $API_URL/ping -H "Authorization: Bearer $TOKEN2"

echo -e "\n\n9. Attempting invalid JSON payload..."
curl -s -X POST $API_URL/sendMessage \
     -H "Authorization: Bearer $TOKEN1" \
     -H "Content-Type: application/json" \
     -d '{"message: "bad format'

echo -e "\n\n=== Stress Test Finished ==="
