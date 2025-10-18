# AIM Sample Agent

Simple example demonstrating the AIM JavaScript SDK.

## Setup

1. **Build the SDK** (first time only):
   ```bash
   cd ../sdks/javascript
   npm install
   npm run build
   cd ../sample-agent
   ```

2. **Install dependencies**:
   ```bash
   npm install
   ```

3. **Run the agent**:
   ```bash
   npm start
   ```

## What it does

1. Registers an agent with AIM using your API key
2. Initializes the AIM client
3. Simulates safe and dangerous operations
4. Shows the dashboard URL to view your agent

## Configuration

Edit `.env` to change:
- `AIM_API_KEY` - Your AIM API key
- `AIM_API_URL` - Backend URL (default: http://localhost:8080)
- `AIM_DASHBOARD_URL` - Dashboard URL (default: http://localhost:3000)


