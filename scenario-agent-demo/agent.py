#!/usr/bin/env python3
"""
CRUD Agent with AIM SDK + Mistral Integration
=======================================================

Features:
- AIM SDK secured CRUD operations
- Mistral AI chat model (mistral-medium-latest)
- FastAPI endpoints to send/receive chat
- Real-time verification & trust scoring

Usage:
    export AIM_API_KEY="your-aim-key"
    export MISTRAL_API_KEY="your-mistral-key"
    uvicorn aim_mistral_crud_agent:app --reload
"""

import os
import sys
from datetime import datetime
from typing import Dict, List, Optional

from dotenv import load_dotenv
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

# Load environment
load_dotenv()

from aim_sdk import secure
from mistralai import Mistral

# -----------------------------------------------------------------------------
# AIM & Mistral Setup
# -----------------------------------------------------------------------------
AIM_API_KEY = os.getenv("AIM_API_KEY")
AIM_API_URL = os.getenv("AIM_API_URL", "http://localhost:8080")
MISTRAL_API_KEY = os.getenv("MISTRAL_API_KEY")

if not AIM_API_KEY:
    raise RuntimeError("❌ AIM_API_KEY environment variable is required.")
if not MISTRAL_API_KEY:
    raise RuntimeError("❌ MISTRAL_API_KEY environment variable is required.")

# Initialize AIM-secured agent
agent_client = secure("mistral-flight-agent", aim_url=AIM_API_URL, api_key=AIM_API_KEY)
print(f"✅ AIM Agent registered: {agent_client.agent_id}")

# Show current capabilities
capabilities = agent_client.get_capabilities()
print(f"📋 Agent capabilities: {capabilities}")

# Initialize Mistral client
mistral = Mistral(api_key=MISTRAL_API_KEY)
MODEL = "mistral-medium-latest"

# -----------------------------------------------------------------------------
# In-memory database (simulate CRUD)
# -----------------------------------------------------------------------------
FLIGHTS_DB: List[Dict] = []
NEXT_ID = 1

# -----------------------------------------------------------------------------
# AIM-Secured CRUD Operations
# -----------------------------------------------------------------------------
@agent_client.perform_action("create_flight", resource="create_flight", context={"risk_level": "low"})
def create_flight_action(title: str, description: str, priority: str = "medium") -> Dict:
    global NEXT_ID
    flight = {
        "id": NEXT_ID,
        "title": title,
        "description": description,
        "priority": priority,
        "status": "pending",
        "created_at": datetime.now().isoformat(),
        "updated_at": datetime.now().isoformat(),
    }
    FLIGHTS_DB.append(flight)
    NEXT_ID += 1
    print(f"🔒 AIM Verified: CREATE flight #{flight['id']}")
    return flight


@agent_client.perform_action("read_flights", resource="flights")
def read_flights(status: Optional[str] = None) -> List[Dict]:
    if status and status != "all":
        return [f for f in FLIGHTS_DB if f["status"] == status]
    return FLIGHTS_DB


@agent_client.perform_action("update_flight", resource="flights")
def update_flight(flight_id: int, status: Optional[str] = None, priority: Optional[str] = None) -> Dict:
    for flight in FLIGHTS_DB:
        if flight["id"] == flight_id:
            if status:
                flight["status"] = status
            if priority:
                flight["priority"] = priority
            flight["updated_at"] = datetime.now().isoformat()
            print(f"🔒 AIM Verified: UPDATE flight #{flight_id}")
            return flight
    raise ValueError(f"Flight with ID {flight_id} not found")


@agent_client.perform_action("delete_flight", resource="flights", context={"risk_level": "high"})
def delete_flight(flight_id: int) -> Dict:
    for i, flight in enumerate(FLIGHTS_DB):
        if flight["id"] == flight_id:
            deleted = FLIGHTS_DB.pop(i)
            print(f"🚨 AIM Verified: DELETE flight #{flight_id}")
            return deleted
    raise ValueError(f"Flight with ID {flight_id} not found")


@agent_client.perform_action("delete_all_flights", resource="delete_all_flights", context={"risk_level": "critical"})
def delete_all_flights() -> Dict:
    global FLIGHTS_DB
    count = len(FLIGHTS_DB)
    FLIGHTS_DB.clear()
    print(f"🚨 AIM Verified: DELETE ALL flights ({count})")
    return {"deleted_count": count, "status": "all_deleted"}

# -----------------------------------------------------------------------------
# FastAPI App
# -----------------------------------------------------------------------------
app = FastAPI(title="Mistral + AIM CRUD Agent", version="1.0")

# Request/Response models
class ChatRequest(BaseModel):
    message: str

class ChatResponse(BaseModel):
    reply: str


@app.post("/chat", response_model=ChatResponse)
def chat_with_agent(req: ChatRequest):
    """
    Send a message to the Mistral-powered agent and get the reply.
    """
    try:
        chat_response = mistral.chat.complete(
            model=MODEL,
            messages=[
                {"role": "system", "content": "You are an AIM-secured CRUD assistant."},
                {"role": "user", "content": req.message},
            ],
        )

        reply = chat_response.choices[0].message.content

        # Basic command interpretation (demo mode)
        lower = req.message.lower()
        if "create" in lower and "flight" in lower:
            create_flight_action("Demo Flight", "Auto-created from /chat endpoint", "medium")
        elif "list" in lower or "show" in lower:
            read_flights("all")
        elif "delete all" in lower:
            delete_all_flights()

        return ChatResponse(reply=reply)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/flights")
def list_flights(status: str = "all"):
    """List all flights."""
    try:
        flights = read_flights(status)
        return {"flights": flights}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
