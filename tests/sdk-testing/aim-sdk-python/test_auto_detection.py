"""
Test script for MCP Auto-Detection

This script demonstrates the new auto-detection capabilities added to the AIM SDK.
It shows how to detect MCP servers and report them to AIM.
"""

import sys
import os

# Add SDK to path
sys.path.insert(0, os.path.dirname(__file__))

from aim_sdk import MCPDetector, auto_detect_mcps


def test_mcp_detector():
    """Test the MCPDetector class."""
    print("=" * 60)
    print("Testing MCPDetector Class")
    print("=" * 60)

    detector = MCPDetector(sdk_version="aim-sdk-python@1.0.0-test")

    # Test Claude config detection
    print("\n1. Testing Claude Config Detection...")
    config_detections = detector.detect_from_claude_config()
    print(f"   Found {len(config_detections)} MCP servers in Claude config")
    for detection in config_detections:
        print(f"   - {detection['mcpServer']} (confidence: {detection['confidence']}%)")

    # Test import detection
    print("\n2. Testing Import Detection...")
    import_detections = detector.detect_from_imports()
    print(f"   Found {len(import_detections)} MCP-related imports")
    for detection in import_detections:
        print(f"   - {detection['mcpServer']} (confidence: {detection['confidence']}%)")

    # Test detect_all
    print("\n3. Testing Combined Detection (detect_all)...")
    all_detections = detector.detect_all()
    print(f"   Total detections: {len(all_detections)}")

    return all_detections


def test_auto_detect_convenience():
    """Test the auto_detect_mcps convenience function."""
    print("\n" + "=" * 60)
    print("Testing auto_detect_mcps() Convenience Function")
    print("=" * 60)

    detections = auto_detect_mcps()
    print(f"\nFound {len(detections)} MCP servers total")

    if detections:
        print("\nDetection Summary:")
        for detection in detections:
            print(f"\nMCP Server: {detection['mcpServer']}")
            print(f"  Method: {detection['detectionMethod']}")
            print(f"  Confidence: {detection['confidence']}%")
            print(f"  SDK Version: {detection['sdkVersion']}")
            print(f"  Timestamp: {detection['timestamp']}")
            if 'details' in detection:
                print(f"  Details: {detection['details']}")

    return detections


def test_detection_format():
    """Verify detection format matches backend expectations."""
    print("\n" + "=" * 60)
    print("Verifying Detection Format")
    print("=" * 60)

    detections = auto_detect_mcps()

    if not detections:
        print("\n⚠️  No detections found - this is normal if no MCP servers are configured")
        return

    print(f"\nValidating {len(detections)} detection(s)...")

    required_fields = ["mcpServer", "detectionMethod", "confidence", "sdkVersion", "timestamp"]

    for i, detection in enumerate(detections):
        print(f"\nDetection {i+1}:")
        missing_fields = [field for field in required_fields if field not in detection]

        if missing_fields:
            print(f"  ❌ Missing required fields: {missing_fields}")
        else:
            print(f"  ✅ All required fields present")

        # Validate confidence range
        confidence = detection.get("confidence", 0)
        if 0 <= confidence <= 100:
            print(f"  ✅ Confidence score valid: {confidence}")
        else:
            print(f"  ❌ Confidence score out of range: {confidence}")

        # Validate detection method
        valid_methods = ["manual", "claude_config", "sdk_import", "sdk_runtime", "direct_api"]
        method = detection.get("detectionMethod", "")
        if method in valid_methods:
            print(f"  ✅ Detection method valid: {method}")
        else:
            print(f"  ⚠️  Detection method not standard: {method}")


def main():
    """Run all tests."""
    print("\n🔍 AIM SDK - MCP Auto-Detection Tests\n")

    try:
        # Test 1: MCPDetector class
        test_mcp_detector()

        # Test 2: Convenience function
        test_auto_detect_convenience()

        # Test 3: Format validation
        test_detection_format()

        print("\n" + "=" * 60)
        print("✅ All tests completed successfully!")
        print("=" * 60)

        print("\n📝 Next Steps:")
        print("1. Create an AIMClient instance")
        print("2. Call client.report_detections(detections)")
        print("3. Check the AIM dashboard for detected MCP servers")

        print("\nExample usage:")
        print("""
from aim_sdk import AIMClient, auto_detect_mcps

# Create client
client = AIMClient(
    agent_id="your-agent-id",
    public_key="your-public-key",
    private_key="your-private-key",
    aim_url="https://aim.example.com"
)

# Auto-detect MCP servers
detections = auto_detect_mcps()

# Report to AIM
result = client.report_detections(detections)
print(f"Processed {result['detectionsProcessed']} detections")
print(f"New MCPs: {result['newMCPs']}")
print(f"Existing MCPs: {result['existingMCPs']}")
        """)

    except Exception as e:
        print(f"\n❌ Test failed with error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == "__main__":
    main()
