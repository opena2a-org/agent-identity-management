/**
 * Test API Key Validation
 */

const crypto = require('crypto');

const apiKeys = [
    'aim_live_Jc1Yj3u',
    'aim_live_yXBE5we'
];

console.log('\n🔍 API Key Hash Test');
console.log('====================\n');

apiKeys.forEach(key => {
    const hash = crypto.createHash('sha256').update(key).digest('base64');
    console.log(`API Key: ${key}`);
    console.log(`SHA-256 Hash: ${hash}`);
    console.log('');
});

console.log('These hashes should match what\'s stored in the database.');
console.log('Check the api_keys table: SELECT id, name, key_hash FROM api_keys;\n');


