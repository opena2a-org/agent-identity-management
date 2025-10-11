from aim_sdk import register_agent

# Register a BRAND NEW agent
import time
timestamp = int(time.time())
print("Registering fresh agent...")
agent = register_agent(
    name=f"test-verification-agent-{timestamp}",
    aim_url="http://localhost:8080",
    display_name="Verification Test Agent",
    description="Testing signature verification",
    force_new=True
)

print(f"\n✅ Registered: {agent.agent_id}")

# Try a simple action
@agent.perform_action("read_database", resource="test_table")
def test_read():
    print("   Inside test_read function!")
    return {"status": "success"}

print("\n🔍 Testing verification...")
try:
    result = test_read()
    print(f"✅ VERIFICATION WORKED! Result: {result}")
except Exception as e:
    print(f"❌ Verification failed: {e}")
    import traceback
    traceback.print_exc()
