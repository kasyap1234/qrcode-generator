#!/bin/bash

CONTAINER_NAME="redis"

echo "Stopping Redis container..."
docker stop $CONTAINER_NAME

echo "Removing Redis container..."
docker rm $CONTAINER_NAME

echo "Redis container stopped and removed."
