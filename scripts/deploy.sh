#!/bin/bash

set-e
APP_NAME=qrgen
PID=$(pgrep "$APP_NAME")
if [-n "$PID"]; then
    echo "Stopping existing app (PID $PID)... "
    kill -9 $PID
    sleep 4
fi

echo "Building app... "
go build -o $APP_NAME main.go

echo "Starting app in background ...."

nohup ./"$APP_NAME" > "$APP_NAME.log" 2>&1 &

echo "APP deployed with PID $PID "$APP_NAME" ,Logs : $APP_NAME.log"
