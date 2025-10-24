// Test script to verify the success page API call works
// Run this in the browser console on http://localhost:3000/dashboard/agents/69b14e60-768c-4af6-aad1-68d243bb264c/success

(async () => {
  const agentId = '69b14e60-768c-4af6-aad1-68d243bb264c';
  const token = localStorage.getItem('auth_token');

  console.log('Testing success page API call...');
  console.log('Agent ID:', agentId);
  console.log('Auth token:', token ? 'Present' : 'Missing');

  if (!token) {
    console.error('❌ No auth token found in localStorage');
    return;
  }

  try {
    const response = await fetch(`http://localhost:8080/api/v1/agents/${agentId}`, {
      method: 'GET',
      headers: {
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/json'
      }
    });

    console.log('Response status:', response.status);

    if (!response.ok) {
      const errorText = await response.text();
      console.error('❌ API call failed:', errorText);
      return;
    }

    const data = await response.json();
    console.log('✅ API call successful!');
    console.log('Agent data:', data);

    // Verify all required fields
    const requiredFields = ['id', 'name', 'display_name', 'public_key', 'status'];
    const missingFields = requiredFields.filter(field => !data[field]);

    if (missingFields.length > 0) {
      console.warn('⚠️ Missing fields:', missingFields);
    } else {
      console.log('✅ All required fields present');
    }

  } catch (error) {
    console.error('❌ Error:', error);
  }
})();
