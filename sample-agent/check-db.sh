#!/bin/bash

echo "🔍 Checking API Keys in Database"
echo "================================="
echo ""

cd ../apps/backend

# Check if we can connect to the database
echo "Checking api_keys table..."
echo ""

psql -h localhost -U postgres -d aim -c "
SELECT 
    id,
    name,
    key_prefix,
    LEFT(key_hash, 20) || '...' as key_hash_preview,
    is_active,
    expires_at,
    created_by,
    organization_id,
    created_at
FROM api_keys 
ORDER BY created_at DESC 
LIMIT 10;
"

echo ""
echo "Expected hashes:"
echo "  aim_live_Jc1Yj3u -> 8UqZxIp02T0ovDjwq3FWLNbPalAensA2Y6zQM2qAIP0="
echo "  aim_live_yXBE5we -> 1LlgywJ/gd5Phy+n7XMXrM0og4aghIaywfOYqW3eUGo="
echo ""

echo "Full hash check for aim_live_yXBE5we:"
psql -h localhost -U postgres -d aim -c "
SELECT 
    name,
    key_prefix,
    key_hash,
    key_hash = '1LlgywJ/gd5Phy+n7XMXrM0og4aghIaywfOYqW3eUGo=' as hash_matches
FROM api_keys 
WHERE key_prefix = 'aim_live_yXBE5we';
"


