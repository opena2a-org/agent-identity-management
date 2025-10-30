#!/usr/bin/env python3
"""
Flight Agent Web UI
===================

A web-based interface for interacting with the AIM-secured Flight Agent.
Features:
- Chat with the Mistral-powered agent
- Manage flights (create, read, update, delete)
- Real-time updates via AJAX
- Modern, responsive design
"""

from flask import Flask, render_template, request, jsonify
import requests
import json
from datetime import datetime
from typing import Dict, List, Optional

app = Flask(__name__)

# API configuration
API_BASE = "http://localhost:8000"

class FlightAgentWeb:
    def __init__(self):
        self.api_base = API_BASE
    
    def chat(self, message: str) -> str:
        """Send a message to the agent and get response"""
        try:
            response = requests.post(
                f"{self.api_base}/chat",
                json={"message": message},
                timeout=30
            )
            
            if response.status_code == 200:
                data = response.json()
                return data.get("reply", "No response")
            else:
                return f"Error: {response.status_code} - {response.text}"
                
        except requests.exceptions.RequestException as e:
            return f"Connection error: {str(e)}"
    
    def list_flights(self, status: str = "all") -> List[Dict]:
        """List all flights"""
        try:
            url = f"{self.api_base}/flights"
            if status != "all":
                url += f"?status={status}"
            
            response = requests.get(url, timeout=10)
            
            if response.status_code == 200:
                data = response.json()
                return data.get("flights", [])
            else:
                return []
                
        except requests.exceptions.RequestException as e:
            return []

# Initialize the agent
agent = FlightAgentWeb()

@app.route('/')
def index():
    """Main page"""
    return render_template('index.html')

@app.route('/api/chat', methods=['POST'])
def api_chat():
    """Chat API endpoint"""
    data = request.get_json()
    message = data.get('message', '')
    
    if not message:
        return jsonify({'error': 'No message provided'}), 400
    
    response = agent.chat(message)
    return jsonify({'reply': response})

@app.route('/api/flights', methods=['GET'])
def api_flights():
    """Get flights API endpoint"""
    status = request.args.get('status', 'all')
    flights = agent.list_flights(status)
    return jsonify({'flights': flights})

@app.route('/api/flights', methods=['POST'])
def api_create_flight():
    """Create flight API endpoint"""
    data = request.get_json()
    title = data.get('title', '')
    description = data.get('description', '')
    priority = data.get('priority', 'medium')
    
    if not title:
        return jsonify({'error': 'Title is required'}), 400
    
    message = f"Create a flight with title '{title}', description '{description}', and priority '{priority}'"
    response = agent.chat(message)
    
    return jsonify({'message': response, 'success': True})

@app.route('/api/flights/<int:flight_id>', methods=['DELETE'])
def api_delete_flight(flight_id):
    """Delete flight API endpoint"""
    message = f"Delete flight with ID {flight_id}"
    response = agent.chat(message)
    
    return jsonify({'message': response, 'success': True})

@app.route('/api/flights/all', methods=['DELETE'])
def api_delete_all_flights():
    """Delete all flights API endpoint"""
    message = "Delete all flights"
    response = agent.chat(message)
    
    return jsonify({'message': response, 'success': True})

if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0', port=5000)
