"""
Example: One-line agent registration with AIM

This demonstrates the simplicity of AIM: enterprise security in one line
"""

from aim_sdk import register_agent

# ============================================================================
# STEP 1: ONE-LINE REGISTRATION
# ============================================================================

print("🚀 Registering agent with AIM...")
print("   (Keys will be generated and stored automatically)\n")

# THIS IS THE MAGIC LINE - that's all you need!
agent = register_agent(
    name="demo-agent-v2",  # Changed name to test new registration
    aim_url="http://localhost:8080",
    display_name="Demo Agent v2",
    description="Demonstration agent for AIM SDK",
    version="1.0.0",
    repository_url="https://github.com/opena2a/agent-identity-management"
)

# ============================================================================
# STEP 2: USE THE AGENT - AUTOMATIC VERIFICATION
# ============================================================================

print("\n✅ Agent ready! Now let's perform some verified actions...\n")

# Example 1: Simple database read
@agent.perform_action("db:read", resource="users_table")  # namespace:action format
def get_user_count():
    """
    This function is automatically verified by AIM before execution.
    AIM creates an audit log entry with cryptographic proof.
    """
    # In a real app, this would query a database
    print("   📊 Querying database...")
    return {"count": 42, "table": "users"}

# Example 2: Sensitive operation with context
@agent.perform_action(
    "config:modify",  # namespace:action format
    resource="system_settings",
    context={"reason": "Update email notifications", "requested_by": "admin"}
)
def update_notification_settings(enabled: bool):
    """
    High-risk action with additional context.
    AIM verifies agent identity and logs all details.
    """
    print(f"   ⚙️  Updating notification settings to: {enabled}")
    return {"notifications_enabled": enabled, "updated_at": "2025-10-07T16:00:00Z"}

# Example 3: High-risk data deletion
@agent.perform_action(
    "data:delete",  # namespace:action format
    resource="test_data"
)
def cleanup_test_data():
    """
    High-risk action that requires elevated trust score.
    AIM may deny this action if agent's trust score is too low.
    """
    print("   🗑️  Cleaning up test data...")
    return {"deleted_rows": 10, "status": "success"}

# ============================================================================
# STEP 3: EXECUTE VERIFIED ACTIONS
# ============================================================================

try:
    print("1️⃣  Getting user count (auto-verified by AIM)...")
    result = get_user_count()
    print(f"   ✅ Result: {result}\n")

    print("2️⃣  Updating notification settings (auto-verified by AIM)...")
    result = update_notification_settings(True)
    print(f"   ✅ Result: {result}\n")

    print("3️⃣  Cleaning up test data (auto-verified by AIM)...")
    result = cleanup_test_data()
    print(f"   ✅ Result: {result}\n")

    print("🎉 All actions completed successfully!")
    print("   Check AIM dashboard for audit logs and trust score updates.\n")

except Exception as e:
    print(f"❌ Error: {e}\n")
    import traceback
    traceback.print_exc()

# ============================================================================
# CREDENTIAL MANAGEMENT
# ============================================================================

print("\n📝 Notes:")
print("   - Credentials saved to: ~/.aim/credentials.json")
print("   - Private key is stored locally (not on server)")
print("   - To re-use credentials, just call register_agent() again")
print("   - Agent name 'demo-agent-v2' will load existing credentials\n")

print("💡 Try running this script again - it will use cached credentials!")
print("   (No new registration will happen, it will reuse existing keys)\n")
