"""
Tests for AIM SDK Risk Detector

Verifies that risk level auto-detection works correctly based on capability patterns.
"""

import pytest
from aim_sdk.risk_detector import (
    detect_risk_level,
    explain_risk_detection,
    get_risk_level_description,
    RISK_LOW,
    RISK_MEDIUM,
    RISK_HIGH,
    RISK_CRITICAL,
)


class TestDetectRiskLevel_ExplicitOverride:
    """Test that explicit overrides take precedence."""

    def test_override_low(self):
        assert detect_risk_level("payment:process", override="low") == RISK_LOW

    def test_override_medium(self):
        assert detect_risk_level("weather:fetch", override="medium") == RISK_MEDIUM

    def test_override_high(self):
        assert detect_risk_level("api:read", override="high") == RISK_HIGH

    def test_override_critical(self):
        assert detect_risk_level("db:read", override="critical") == RISK_CRITICAL

    def test_invalid_override_falls_through(self):
        # Invalid override should be ignored, fall through to detection
        assert detect_risk_level("weather:fetch", override="invalid") == RISK_LOW


class TestDetectRiskLevel_SpecificCapabilityMap:
    """Test specific capability mappings (highest priority after override)."""

    # User operations
    def test_user_delete_is_critical(self):
        assert detect_risk_level("user:delete") == RISK_CRITICAL

    def test_user_impersonate_is_critical(self):
        assert detect_risk_level("user:impersonate") == RISK_CRITICAL

    def test_user_admin_is_critical(self):
        assert detect_risk_level("user:admin") == RISK_CRITICAL

    # Auth operations
    def test_auth_login_is_medium(self):
        assert detect_risk_level("auth:login") == RISK_MEDIUM

    def test_auth_logout_is_low(self):
        assert detect_risk_level("auth:logout") == RISK_LOW

    def test_auth_reset_password_is_high(self):
        assert detect_risk_level("auth:reset_password") == RISK_HIGH

    def test_auth_change_password_is_high(self):
        assert detect_risk_level("auth:change_password") == RISK_HIGH

    # API calls
    def test_api_call_is_low(self):
        assert detect_risk_level("api:call") == RISK_LOW

    def test_api_webhook_is_medium(self):
        assert detect_risk_level("api:webhook") == RISK_MEDIUM

    def test_api_external_is_high(self):
        assert detect_risk_level("api:external") == RISK_HIGH

    # Database
    def test_db_read_is_low(self):
        assert detect_risk_level("db:read") == RISK_LOW

    def test_db_query_is_low(self):
        assert detect_risk_level("db:query") == RISK_LOW

    def test_db_write_is_medium(self):
        assert detect_risk_level("db:write") == RISK_MEDIUM

    def test_db_delete_is_high(self):
        assert detect_risk_level("db:delete") == RISK_HIGH

    def test_db_admin_is_critical(self):
        assert detect_risk_level("db:admin") == RISK_CRITICAL

    def test_db_drop_is_critical(self):
        assert detect_risk_level("db:drop") == RISK_CRITICAL

    # File operations
    def test_file_read_is_low(self):
        assert detect_risk_level("file:read") == RISK_LOW

    def test_file_write_is_medium(self):
        assert detect_risk_level("file:write") == RISK_MEDIUM

    def test_file_delete_is_high(self):
        assert detect_risk_level("file:delete") == RISK_HIGH

    def test_file_execute_is_critical(self):
        assert detect_risk_level("file:execute") == RISK_CRITICAL

    # MCP operations
    def test_mcp_tool_use_is_medium(self):
        assert detect_risk_level("mcp:tool_use") == RISK_MEDIUM

    def test_mcp_resource_read_is_low(self):
        assert detect_risk_level("mcp:resource_read") == RISK_LOW

    # System operations
    def test_system_read_is_medium(self):
        assert detect_risk_level("system:read") == RISK_MEDIUM

    def test_system_exec_is_critical(self):
        assert detect_risk_level("system:exec") == RISK_CRITICAL

    def test_system_admin_is_critical(self):
        assert detect_risk_level("system:admin") == RISK_CRITICAL


class TestDetectRiskLevel_ActionPatterns:
    """Test action-based pattern matching."""

    # Low risk actions
    def test_custom_read_is_low(self):
        assert detect_risk_level("custom:read") == RISK_LOW

    def test_custom_fetch_is_low(self):
        assert detect_risk_level("custom:fetch") == RISK_LOW

    def test_custom_get_is_low(self):
        assert detect_risk_level("custom:get") == RISK_LOW

    def test_custom_list_is_low(self):
        assert detect_risk_level("custom:list") == RISK_LOW

    def test_custom_search_is_low(self):
        assert detect_risk_level("custom:search") == RISK_LOW

    def test_custom_query_is_low(self):
        assert detect_risk_level("custom:query") == RISK_LOW

    def test_custom_view_is_low(self):
        assert detect_risk_level("custom:view") == RISK_LOW

    def test_custom_check_is_low(self):
        assert detect_risk_level("custom:check") == RISK_LOW

    def test_custom_validate_is_low(self):
        assert detect_risk_level("custom:validate") == RISK_LOW

    # Medium risk actions
    def test_custom_write_is_medium(self):
        assert detect_risk_level("custom:write") == RISK_MEDIUM

    def test_custom_update_is_medium(self):
        assert detect_risk_level("custom:update") == RISK_MEDIUM

    def test_custom_create_is_medium(self):
        assert detect_risk_level("custom:create") == RISK_MEDIUM

    def test_custom_modify_is_medium(self):
        assert detect_risk_level("custom:modify") == RISK_MEDIUM

    def test_custom_edit_is_medium(self):
        assert detect_risk_level("custom:edit") == RISK_MEDIUM

    def test_custom_set_is_medium(self):
        assert detect_risk_level("custom:set") == RISK_MEDIUM

    def test_custom_save_is_medium(self):
        assert detect_risk_level("custom:save") == RISK_MEDIUM

    def test_custom_upload_is_medium(self):
        assert detect_risk_level("custom:upload") == RISK_MEDIUM

    # High risk actions
    def test_custom_delete_is_high(self):
        assert detect_risk_level("custom:delete") == RISK_HIGH

    def test_custom_remove_is_high(self):
        assert detect_risk_level("custom:remove") == RISK_HIGH

    def test_custom_send_is_high(self):
        assert detect_risk_level("custom:send") == RISK_HIGH

    def test_custom_execute_is_high(self):
        assert detect_risk_level("custom:execute") == RISK_HIGH

    def test_custom_run_is_high(self):
        assert detect_risk_level("custom:run") == RISK_HIGH

    def test_custom_invoke_is_high(self):
        assert detect_risk_level("custom:invoke") == RISK_HIGH

    def test_custom_export_is_high(self):
        assert detect_risk_level("custom:export") == RISK_HIGH

    def test_custom_transfer_is_high(self):
        assert detect_risk_level("custom:transfer") == RISK_HIGH

    # Critical risk actions
    def test_custom_process_is_critical(self):
        assert detect_risk_level("custom:process") == RISK_CRITICAL

    def test_custom_refund_is_critical(self):
        assert detect_risk_level("custom:refund") == RISK_CRITICAL

    def test_custom_charge_is_critical(self):
        assert detect_risk_level("custom:charge") == RISK_CRITICAL

    def test_custom_pay_is_critical(self):
        assert detect_risk_level("custom:pay") == RISK_CRITICAL

    def test_custom_approve_is_critical(self):
        assert detect_risk_level("custom:approve") == RISK_CRITICAL

    def test_custom_admin_is_critical(self):
        assert detect_risk_level("custom:admin") == RISK_CRITICAL

    def test_custom_impersonate_is_critical(self):
        assert detect_risk_level("custom:impersonate") == RISK_CRITICAL

    def test_custom_destroy_is_critical(self):
        assert detect_risk_level("custom:destroy") == RISK_CRITICAL

    def test_custom_purge_is_critical(self):
        assert detect_risk_level("custom:purge") == RISK_CRITICAL

    def test_custom_wipe_is_critical(self):
        assert detect_risk_level("custom:wipe") == RISK_CRITICAL


class TestDetectRiskLevel_NamespacePatterns:
    """Test namespace-based pattern matching."""

    # Critical risk namespaces
    def test_payment_namespace(self):
        assert detect_risk_level("payment:unknown") == RISK_CRITICAL

    def test_admin_namespace(self):
        assert detect_risk_level("admin:unknown") == RISK_CRITICAL

    def test_system_namespace(self):
        assert detect_risk_level("system:unknown") == RISK_CRITICAL

    def test_billing_namespace(self):
        assert detect_risk_level("billing:unknown") == RISK_CRITICAL

    def test_finance_namespace(self):
        assert detect_risk_level("finance:unknown") == RISK_CRITICAL

    # High risk namespaces
    def test_email_namespace(self):
        assert detect_risk_level("email:unknown") == RISK_HIGH

    def test_notification_namespace(self):
        assert detect_risk_level("notification:unknown") == RISK_HIGH

    def test_sms_namespace(self):
        assert detect_risk_level("sms:unknown") == RISK_HIGH

    def test_user_namespace(self):
        assert detect_risk_level("user:unknown") == RISK_HIGH

    def test_auth_namespace(self):
        assert detect_risk_level("auth:unknown") == RISK_HIGH

    def test_secret_namespace(self):
        assert detect_risk_level("secret:unknown") == RISK_HIGH

    def test_credential_namespace(self):
        assert detect_risk_level("credential:unknown") == RISK_HIGH

    # Medium risk namespaces
    def test_db_namespace(self):
        assert detect_risk_level("db:unknown") == RISK_MEDIUM

    def test_database_namespace(self):
        assert detect_risk_level("database:unknown") == RISK_MEDIUM

    def test_file_namespace(self):
        assert detect_risk_level("file:unknown") == RISK_MEDIUM

    def test_storage_namespace(self):
        assert detect_risk_level("storage:unknown") == RISK_MEDIUM

    def test_cache_namespace(self):
        assert detect_risk_level("cache:unknown") == RISK_MEDIUM

    # Low risk namespaces
    def test_api_namespace(self):
        assert detect_risk_level("api:unknown") == RISK_LOW

    def test_weather_namespace(self):
        assert detect_risk_level("weather:unknown") == RISK_LOW

    def test_search_namespace(self):
        assert detect_risk_level("search:unknown") == RISK_LOW

    def test_geocode_namespace(self):
        assert detect_risk_level("geocode:unknown") == RISK_LOW

    def test_translate_namespace(self):
        assert detect_risk_level("translate:unknown") == RISK_LOW

    def test_time_namespace(self):
        assert detect_risk_level("time:unknown") == RISK_LOW

    def test_math_namespace(self):
        assert detect_risk_level("math:unknown") == RISK_LOW

    def test_util_namespace(self):
        assert detect_risk_level("util:unknown") == RISK_LOW


class TestDetectRiskLevel_CombinedActionAndNamespace:
    """Test that higher risk wins when action and namespace conflict."""

    def test_payment_read_uses_namespace(self):
        # Namespace critical > action low
        assert detect_risk_level("payment:read") == RISK_CRITICAL

    def test_admin_fetch_uses_namespace(self):
        # Namespace critical > action low
        assert detect_risk_level("admin:fetch") == RISK_CRITICAL

    def test_email_get_uses_namespace(self):
        # Namespace high > action low
        assert detect_risk_level("email:get") == RISK_HIGH

    def test_weather_delete_uses_action(self):
        # Action high > namespace low
        assert detect_risk_level("weather:delete") == RISK_HIGH

    def test_api_process_uses_action(self):
        # Action critical > namespace low
        assert detect_risk_level("api:process") == RISK_CRITICAL


class TestDetectRiskLevel_CaseInsensitive:
    """Test case insensitivity."""

    def test_uppercase(self):
        assert detect_risk_level("PAYMENT:PROCESS") == RISK_CRITICAL

    def test_mixed_case(self):
        assert detect_risk_level("Payment:Process") == RISK_CRITICAL

    def test_mixed_case_db_read(self):
        assert detect_risk_level("DB:Read") == RISK_LOW

    def test_uppercase_weather(self):
        assert detect_risk_level("WEATHER:FETCH") == RISK_LOW


class TestDetectRiskLevel_EdgeCases:
    """Test edge cases."""

    def test_empty_capability(self):
        assert detect_risk_level("") == RISK_MEDIUM

    def test_no_colon(self):
        assert detect_risk_level("justaction") == RISK_MEDIUM

    def test_whitespace_trimmed(self):
        assert detect_risk_level("  weather:fetch  ") == RISK_LOW

    def test_unknown_defaults_to_medium(self):
        assert detect_risk_level("foo:bar") == RISK_MEDIUM


class TestExplainRiskDetection:
    """Test the explain_risk_detection function."""

    def test_specific_mapping(self):
        result = explain_risk_detection("user:delete")
        assert result["risk_level"] == RISK_CRITICAL
        assert result["source"] == "specific_capability_map"

    def test_action_mapping(self):
        result = explain_risk_detection("custom:delete")
        assert result["risk_level"] == RISK_HIGH
        assert result["source"] == "action_map"

    def test_namespace_mapping(self):
        result = explain_risk_detection("payment:unknown")
        assert result["risk_level"] == RISK_CRITICAL
        assert result["source"] == "namespace_map"

    def test_action_and_namespace(self):
        result = explain_risk_detection("payment:read")
        assert result["risk_level"] == RISK_CRITICAL
        assert result["source"] == "action_and_namespace"

    def test_default(self):
        result = explain_risk_detection("foo:bar")
        assert result["risk_level"] == RISK_MEDIUM
        assert result["source"] == "default"

    def test_invalid_format(self):
        result = explain_risk_detection("nocolon")
        assert result["risk_level"] == RISK_MEDIUM
        assert result["source"] == "default"


class TestGetRiskLevelDescription:
    """Test risk level descriptions."""

    def test_low(self):
        desc = get_risk_level_description(RISK_LOW)
        assert "audit" in desc.lower()

    def test_medium(self):
        desc = get_risk_level_description(RISK_MEDIUM)
        assert "pattern" in desc.lower() or "monitored" in desc.lower()

    def test_high(self):
        desc = get_risk_level_description(RISK_HIGH)
        assert "alert" in desc.lower()

    def test_critical(self):
        desc = get_risk_level_description(RISK_CRITICAL)
        assert "admin" in desc.lower() or "approval" in desc.lower()

    def test_invalid(self):
        desc = get_risk_level_description("invalid")
        assert "unknown" in desc.lower()


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
