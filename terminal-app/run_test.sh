#!/bin/bash

# Configuration
export SYSTEM_RESPONDER_TYPE=mock
export SYSTEM_TICK_SECONDS=1
export SYSTEM_TEMP_SLEEP_DELAY_MINS=0.02
export SYSTEM_TRUE_SLEEP_DELAY_MINS=0.05
export DEBUG=1
export WEB_USER=admin
export WEB_PASS=password
export PORT=8080
export JWT_SECRET=testsecret
export SYSTEM_NO_INTERFACE=true

echo "[Test] Building app..."
go build -o test_app ./main.go

echo "[Test] Starting background app..."
./test_app -newSession > debug.log 2>&1 &
APP_PID=$!

echo "[Test] App PID: $APP_PID"
echo "[Test] Waiting 3 seconds for server to start..."
sleep 3

echo "[Test] Authenticating..."
TOKEN=$(curl -s -X POST http://localhost:8080/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin", "password":"password"}' | grep -o '"token":"[^"]*' | grep -o '[^"]*$')

if [ -z "$TOKEN" ]; then
    echo "[Test] Failed to authenticate!"
    kill $APP_PID
    cat debug.log
    exit 1
fi

echo "[Test] Sending mock message via API..."
curl -s -X POST http://localhost:8080/sendMessage \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"message":"testing the system loop"}'

echo ""
echo "[Test] Waiting 10 seconds for events to process (reactor, responder, sleeps)..."
sleep 10

echo "[Test] Shutting down app..."
kill $APP_PID

echo "================ APP LOGS ================"
cat debug.log
echo "=========================================="
