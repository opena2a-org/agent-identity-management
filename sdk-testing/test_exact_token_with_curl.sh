#!/bin/bash

TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiYTAwMDAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMDAwMDAwMDAyIiwib3JnYW5pemF0aW9uX2lkIjoiYTAwMDAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMDAwMDAwMDAxIiwiZW1haWwiOiJhZG1pbkBvcGVuYTJhLm9yZyIsInJvbGUiOiJhZG1pbiIsImlzcyI6ImFnZW50LWlkZW50aXR5LW1hbmFnZW1lbnQtc2RrIiwic3ViIjoiYTAwMDAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMDAwMDAwMDAyIiwiZXhwIjoxNzY4OTc4NjYwLCJuYmYiOjE3NjEyMDI2NjAsImlhdCI6MTc2MTIwMjY2MCwianRpIjoiNzAyZjM0OTMtZWRjYy00MjQzLTg0M2QtNzA1MDE1OGMyZWQ4In0.SLfzr3U60MRCR6HJ3Mkyj_clANXKc7wqGlCdpR4FfXQ"

echo "Testing EXACT token from credentials.json with curl..."
echo "Token ID should be: 702f3493-edcc-4243-843d-7050158c2ed8"
echo ""

curl -X POST \
  https://aim-prod-backend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\": \"$TOKEN\"}" \
  -v 2>&1 | head -50
