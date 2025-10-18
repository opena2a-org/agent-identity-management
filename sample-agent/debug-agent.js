/**
 * Debug version to see what's being sent
 */

const axios = require('axios');

const AIM_API_KEY = 'aim_live_k2frcq38yTMq0E59EjGCeX8CGAVzoTVa7GBU3Fe-GNM=';
const AIM_API_URL = 'http://localhost:8080';

async function testDirectRegistration() {
    console.log('\n🔍 Debug: Direct API Call Test');
    console.log('================================\n');

    const payload = {
        name: 'debug-test-agent',
        display_name: 'Debug Test Agent',
        description: 'Testing direct API call',
        agent_type: 'ai_agent',
        version: '1.0.0',
        repository_url: '',
        documentation_url: '',
    };

    console.log('API URL:', AIM_API_URL);
    console.log('Endpoint:', `${AIM_API_URL}/api/v1/public/agents/register`);
    console.log('API Key:', AIM_API_KEY);
    console.log('Payload:', JSON.stringify(payload, null, 2));
    console.log('\nHeaders:');
    console.log('  Content-Type: application/json');
    console.log('  X-AIM-API-Key:', AIM_API_KEY);

    try {
        console.log('\n📤 Sending request...\n');
        
        const response = await axios.post(
            `${AIM_API_URL}/api/v1/public/agents/register`,
            payload,
            {
                headers: {
                    'Content-Type': 'application/json',
                    'X-AIM-API-Key': AIM_API_KEY,
                }
            }
        );

        console.log('✅ Success!');
        console.log('Response:', JSON.stringify(response.data, null, 2));

    } catch (error) {
        console.error('❌ Error:', error.message);
        if (error.response) {
            console.error('Status:', error.response.status);
            console.error('Response:', JSON.stringify(error.response.data, null, 2));
            console.error('Response Headers:', JSON.stringify(error.response.headers, null, 2));
        }
        if (error.config) {
            console.error('\nRequest Config:');
            console.error('  URL:', error.config.url);
            console.error('  Method:', error.config.method);
            console.error('  Headers:', JSON.stringify(error.config.headers, null, 2));
            console.error('  Data:', error.config.data);
        }
    }
}

testDirectRegistration();

