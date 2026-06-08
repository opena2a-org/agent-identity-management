"""
Centralized Credential Management for AIM SDK.

This module provides a clean separation between:
1. SDK credentials (OAuth tokens for authenticating the SDK itself)
2. Agent credentials (Ed25519 keys for registered agents)

Directory Structure:
    ~/.aim/
    ├── sdk_credentials.json      # SDK OAuth tokens
    ├── agents/
    │   ├── my-agent.json         # Agent "my-agent" keys
    │   └── other-agent.json      # Agent "other-agent" keys
    └── credentials.json          # LEGACY (auto-migrated)

This design ensures:
- No credential collisions (SDK auth vs Agent auth)
- Per-agent isolation (each agent has its own file)
- Clear error messages when wrong credential type found
- Automatic migration from legacy format
"""

import os
import json
import shutil
from pathlib import Path
from typing import Optional, Dict, Any, Tuple
from datetime import datetime

# Security logging for SOC monitoring
from .security_logging import security_logger, CredEventType


# =============================================================================
# PATH CONSTANTS
# =============================================================================

AIM_DIR = Path.home() / ".aim"
SDK_CREDENTIALS_FILE = AIM_DIR / "sdk_credentials.json"
AGENTS_DIR = AIM_DIR / "agents"
LEGACY_CREDENTIALS_FILE = AIM_DIR / "credentials.json"

# Schema version for future evolution
SCHEMA_VERSION = "1.0"


# =============================================================================
# CREDENTIAL TYPE DETECTION
# =============================================================================

class CredentialType:
    """Types of credentials in the AIM ecosystem."""
    SDK_OAUTH = "sdk_oauth"      # SDK authentication (refreshToken, sdkTokenId)
    AGENT = "agent"              # Agent keys (agentId, privateKey)
    UNKNOWN = "unknown"          # Unrecognized format


def detect_credential_type(data: Dict[str, Any]) -> str:
    """
    Detect the type of credentials from their structure.

    Args:
        data: Parsed JSON credential data

    Returns:
        CredentialType constant
    """
    # SDK OAuth credentials have refreshToken
    if "refreshToken" in data or "refresh_token" in data:
        return CredentialType.SDK_OAUTH

    # Agent credentials have agent_id or agentId and private_key or privateKey
    has_agent_id = "agent_id" in data or "agentId" in data
    has_private_key = "private_key" in data or "privateKey" in data
    if has_agent_id or has_private_key:
        return CredentialType.AGENT

    # Check for nested agent credentials (old format: {"agent-name": {...}})
    for key, value in data.items():
        if isinstance(value, dict):
            if "agent_id" in value or "private_key" in value:
                return CredentialType.AGENT

    return CredentialType.UNKNOWN


# =============================================================================
# SDK CREDENTIALS (OAuth tokens for SDK authentication)
# =============================================================================

def get_sdk_credentials_path() -> Path:
    """Get the path to SDK credentials file."""
    return SDK_CREDENTIALS_FILE


def load_sdk_credentials() -> Optional[Dict[str, Any]]:
    """
    Load SDK OAuth credentials with intelligent fallback and migration.

    Lookup order:
    1. ~/.aim/sdk_credentials.json (new location)
    2. ~/.aim/credentials.json (legacy - migrate if SDK type)
    3. SDK package .aim/sdk_credentials.json (fresh download)

    Returns:
        SDK credentials dict or None if not found
    """
    # 1. Check new location first
    if SDK_CREDENTIALS_FILE.exists():
        try:
            with open(SDK_CREDENTIALS_FILE, 'r') as f:
                data = json.load(f)
            if detect_credential_type(data) == CredentialType.SDK_OAUTH:
                security_logger.log_credential_event(
                    CredEventType.CREDENTIAL_LOADED,
                    credential_type="sdk_oauth",
                    success=True,
                    details={"source": "sdk_credentials.json", "token_id": data.get("sdkTokenId", "")[:8] + "..."}
                )
                return data
        except Exception as e:
            security_logger.log_credential_event(
                CredEventType.CREDENTIAL_LOADED,
                credential_type="sdk_oauth",
                success=False,
                error=str(e),
                details={"source": "sdk_credentials.json"}
            )

    # 2. Check legacy location and migrate if SDK type
    if LEGACY_CREDENTIALS_FILE.exists():
        try:
            with open(LEGACY_CREDENTIALS_FILE, 'r') as f:
                data = json.load(f)

            cred_type = detect_credential_type(data)

            if cred_type == CredentialType.SDK_OAUTH:
                # Migrate to new location
                _migrate_sdk_credentials(data)
                security_logger.log_credential_event(
                    CredEventType.CREDENTIAL_MIGRATED,
                    credential_type="sdk_oauth",
                    success=True,
                    details={"from": "credentials.json", "to": "sdk_credentials.json"}
                )
                return data

            elif cred_type == CredentialType.AGENT:
                # Agent credentials in legacy location - migrate them too
                _migrate_legacy_agent_credentials(data)
                security_logger.log_credential_event(
                    CredEventType.CREDENTIAL_TYPE_MISMATCH,
                    credential_type="agent",
                    success=False,
                    details={"expected": "sdk_oauth", "found": "agent", "source": "credentials.json"}
                )
                # Fall through to check SDK package

        except Exception as e:
            security_logger.log_credential_event(
                CredEventType.CREDENTIAL_LOADED,
                credential_type="sdk_oauth",
                success=False,
                error=str(e),
                details={"source": "credentials.json (legacy)"}
            )

    # 3. Check SDK package for fresh download
    found = _find_sdk_package_credentials()
    if found:
        sdk_package_creds, source_path = found
        # Install to home directory
        _install_sdk_credentials(sdk_package_creds, source_path)
        security_logger.log_credential_event(
            CredEventType.CREDENTIAL_CREATED,
            credential_type="sdk_oauth",
            success=True,
            details={"source": str(source_path), "installed_to": "sdk_credentials.json"}
        )
        return sdk_package_creds

    security_logger.log_credential_event(
        CredEventType.CREDENTIAL_LOADED,
        credential_type="sdk_oauth",
        success=False,
        error="SDK credentials not found in any location"
    )
    return None


def save_sdk_credentials(credentials: Dict[str, Any]) -> bool:
    """
    Save SDK OAuth credentials to the proper location.

    Args:
        credentials: SDK credentials dict

    Returns:
        True if saved successfully
    """
    try:
        AIM_DIR.mkdir(parents=True, exist_ok=True)

        # Add schema info
        credentials["schemaVersion"] = SCHEMA_VERSION
        credentials["type"] = CredentialType.SDK_OAUTH

        with open(SDK_CREDENTIALS_FILE, 'w') as f:
            json.dump(credentials, f, indent=2)
        os.chmod(SDK_CREDENTIALS_FILE, 0o600)

        security_logger.log_credential_event(
            CredEventType.CREDENTIAL_SAVED,
            credential_type="sdk_oauth",
            success=True,
            details={"path": str(SDK_CREDENTIALS_FILE), "permissions": "0600"}
        )
        return True
    except Exception as e:
        security_logger.log_credential_event(
            CredEventType.CREDENTIAL_SAVED,
            credential_type="sdk_oauth",
            success=False,
            error=str(e)
        )
        print(f"⚠️  Failed to save SDK credentials: {e}")
        return False


def _find_sdk_package_credentials() -> Optional[Tuple[Dict[str, Any], Path]]:
    """
    Find SDK credentials in the installed package (for fresh downloads).

    Search order:
    1. Current working directory .aim/sdk_credentials.json
    2. SDK package root .aim/sdk_credentials.json
    3. Legacy locations with credentials.json
    """
    search_paths = []

    # 1. Current working directory (when user runs from extracted SDK folder)
    cwd = Path.cwd()
    search_paths.append(cwd / ".aim" / "sdk_credentials.json")
    search_paths.append(cwd / ".aim" / "credentials.json")

    # 2. SDK package installation directory
    try:
        import aim_sdk
        sdk_root = Path(aim_sdk.__file__).parent.parent
        search_paths.append(sdk_root / ".aim" / "sdk_credentials.json")
        search_paths.append(sdk_root / ".aim" / "credentials.json")
    except Exception:
        pass

    # 3. Parent directories (if running from subdirectory of SDK)
    for parent in cwd.parents:
        aim_dir = parent / ".aim"
        if aim_dir.exists():
            search_paths.append(aim_dir / "sdk_credentials.json")
            search_paths.append(aim_dir / "credentials.json")
            break  # Only check first parent with .aim

    # Search all paths
    for path in search_paths:
        if path.exists():
            try:
                with open(path, 'r') as f:
                    data = json.load(f)
                if detect_credential_type(data) == CredentialType.SDK_OAUTH:
                    return data, path
            except Exception:
                continue

    return None


def _install_sdk_credentials(credentials: Dict[str, Any], source_path: Optional[Path] = None) -> None:
    """Adopt SDK credentials discovered on this machine into the home directory.

    The credentials are read from the first existing path in the search order
    (CWD, the installed package dir, or the nearest ancestor ``.aim/``) -- they
    are NOT baked into the published package. The home location is the long-term
    source of truth; on subsequent runs the SDK reads from there first and
    ignores the discovered file (#178).
    """
    try:
        token_id = credentials.get("sdkTokenId") or credentials.get("sdk_token_id") or ""
        token_id_short = (token_id[:8] + "...") if token_id else "unknown"
        # Name the ACTUAL source path so this line is never mistaken for a
        # credential shipped inside the package -- it is not (see docstring).
        # Audit #37: the silent install path was how stale tokens crept into the
        # home file across demo-prep runs, so the adoption stays visible.
        origin = str(source_path) if source_path else "a discovered .aim directory"
        print(
            f"INFO: Adopting existing SDK credentials found at {origin} "
            f"(token {token_id_short}). Long-term source: {SDK_CREDENTIALS_FILE}."
        )
        save_sdk_credentials(credentials)
        print(f"SDK credentials installed to {SDK_CREDENTIALS_FILE}")
    except Exception as e:
        print(f"Failed to install SDK credentials: {e}")


def _migrate_sdk_credentials(credentials: Dict[str, Any]) -> None:
    """Migrate SDK credentials from legacy to new location."""
    try:
        save_sdk_credentials(credentials)
        print(f"✅ SDK credentials migrated to {SDK_CREDENTIALS_FILE}")
    except Exception:
        pass


# =============================================================================
# AGENT CREDENTIALS (Ed25519 keys for registered agents)
# =============================================================================

def get_agent_credentials_path(agent_name: str) -> Path:
    """Get the path to an agent's credentials file."""
    # Sanitize agent name for filesystem
    safe_name = "".join(c if c.isalnum() or c in "-_" else "_" for c in agent_name)
    return AGENTS_DIR / f"{safe_name}.json"


def load_agent_credentials(agent_name: str) -> Optional[Dict[str, Any]]:
    """
    Load credentials for a specific agent.

    Lookup order:
    1. ~/.aim/agents/{agent_name}.json (new location)
    2. ~/.aim/credentials.json with nested agent data (legacy)

    Args:
        agent_name: Name of the agent

    Returns:
        Agent credentials dict or None if not found
    """
    # 1. Check new location
    agent_file = get_agent_credentials_path(agent_name)
    if agent_file.exists():
        try:
            with open(agent_file, 'r') as f:
                data = json.load(f)
            agent_id = data.get("agent_id", "")
            security_logger.log_credential_event(
                CredEventType.CREDENTIAL_LOADED,
                credential_type="agent",
                agent_name=agent_name,
                success=True,
                details={"source": f"agents/{agent_name}.json", "agent_id": (agent_id[:8] + "...") if agent_id else None}
            )
            return data
        except Exception as e:
            security_logger.log_credential_event(
                CredEventType.CREDENTIAL_LOADED,
                credential_type="agent",
                agent_name=agent_name,
                success=False,
                error=str(e)
            )

    # 2. Check legacy location (nested format)
    if LEGACY_CREDENTIALS_FILE.exists():
        try:
            with open(LEGACY_CREDENTIALS_FILE, 'r') as f:
                data = json.load(f)

            # Check for nested agent data: {"agent-name": {...}}
            if agent_name in data and isinstance(data[agent_name], dict):
                agent_data = data[agent_name]
                # Migrate to new location
                save_agent_credentials(agent_name, agent_data)
                security_logger.log_credential_event(
                    CredEventType.CREDENTIAL_MIGRATED,
                    credential_type="agent",
                    agent_name=agent_name,
                    success=True,
                    details={"from": "credentials.json (nested)", "to": f"agents/{agent_name}.json"}
                )
                return agent_data

            # Check for flat agent data with matching name
            if data.get("name") == agent_name or data.get("agent_name") == agent_name:
                save_agent_credentials(agent_name, data)
                security_logger.log_credential_event(
                    CredEventType.CREDENTIAL_MIGRATED,
                    credential_type="agent",
                    agent_name=agent_name,
                    success=True,
                    details={"from": "credentials.json (flat)", "to": f"agents/{agent_name}.json"}
                )
                return data

        except Exception:
            pass

    return None


def save_agent_credentials(agent_name: str, credentials: Dict[str, Any]) -> bool:
    """
    Save credentials for a specific agent.

    Args:
        agent_name: Name of the agent
        credentials: Agent credentials dict

    Returns:
        True if saved successfully
    """
    try:
        AGENTS_DIR.mkdir(parents=True, exist_ok=True)

        # Add schema info
        credentials["schemaVersion"] = SCHEMA_VERSION
        credentials["type"] = CredentialType.AGENT
        credentials["name"] = agent_name

        agent_file = get_agent_credentials_path(agent_name)
        with open(agent_file, 'w') as f:
            json.dump(credentials, f, indent=2)
        os.chmod(agent_file, 0o600)

        agent_id = credentials.get("agent_id", "")
        security_logger.log_credential_event(
            CredEventType.CREDENTIAL_SAVED,
            credential_type="agent",
            agent_name=agent_name,
            success=True,
            details={
                "path": str(agent_file),
                "permissions": "0600",
                "agent_id": (agent_id[:8] + "...") if agent_id else None
            }
        )
        return True
    except Exception as e:
        security_logger.log_credential_event(
            CredEventType.CREDENTIAL_SAVED,
            credential_type="agent",
            agent_name=agent_name,
            success=False,
            error=str(e)
        )
        print(f"⚠️  Failed to save agent credentials: {e}")
        return False


def delete_agent_credentials(agent_name: str) -> bool:
    """Delete credentials for a specific agent."""
    try:
        agent_file = get_agent_credentials_path(agent_name)
        if agent_file.exists():
            agent_file.unlink()
            security_logger.log_credential_event(
                CredEventType.CREDENTIAL_DELETED,
                credential_type="agent",
                agent_name=agent_name,
                success=True,
                details={"path": str(agent_file)}
            )
        return True
    except Exception as e:
        security_logger.log_credential_event(
            CredEventType.CREDENTIAL_DELETED,
            credential_type="agent",
            agent_name=agent_name,
            success=False,
            error=str(e)
        )
        return False


def list_agent_credentials() -> list:
    """List all agents with saved credentials."""
    agents = []
    if AGENTS_DIR.exists():
        for f in AGENTS_DIR.glob("*.json"):
            try:
                with open(f, 'r') as file:
                    data = json.load(file)
                agents.append({
                    "name": data.get("name", f.stem),
                    "agent_id": data.get("agent_id") or data.get("agentId"),
                    "file": str(f)
                })
            except Exception:
                pass
    return agents


def _migrate_legacy_agent_credentials(data: Dict[str, Any]) -> None:
    """Migrate agent credentials from legacy format to new per-agent files."""
    try:
        AGENTS_DIR.mkdir(parents=True, exist_ok=True)

        # Check for nested format: {"agent-name": {...}}
        for key, value in data.items():
            if isinstance(value, dict) and ("agent_id" in value or "private_key" in value):
                save_agent_credentials(key, value)
                print(f"✅ Agent '{key}' credentials migrated to {AGENTS_DIR}")

        # Check for flat format with name field
        if "agent_id" in data or "private_key" in data:
            name = data.get("name") or data.get("agent_name") or "default"
            save_agent_credentials(name, data)
            print(f"✅ Agent '{name}' credentials migrated to {AGENTS_DIR}")

    except Exception as e:
        print(f"⚠️  Failed to migrate agent credentials: {e}")


# =============================================================================
# ERROR MESSAGES
# =============================================================================

def print_sdk_credentials_not_found_error(aim_url: str = "http://localhost:8080") -> None:
    """Print a clear error message when SDK credentials are not found."""
    print()
    print("=" * 72)
    print("❌ SDK CREDENTIALS NOT FOUND")
    print("=" * 72)
    print()
    print("No SDK authentication credentials found.")
    print()
    print("The SDK needs OAuth credentials to authenticate with the AIM server.")
    print("These are different from agent credentials (which store agent keys).")
    print()
    print("TO FIX:")
    print(f"  1. Visit your AIM dashboard: {aim_url}")
    print("  2. Go to Settings → SDK Downloads")
    print("  3. Download the Python SDK")
    print("  4. Extract and install: pip install -e aim-sdk-python/")
    print()
    print("Your downloaded SDK includes authentication credentials.")
    print("=" * 72)
    print()


def print_wrong_credential_type_error(found_type: str, agent_name: str = None, aim_url: str = "http://localhost:8080") -> None:
    """Print a clear error message when wrong credential type is found."""
    print()
    print("=" * 72)
    print("⚠️  WRONG CREDENTIAL TYPE")
    print("=" * 72)
    print()

    if found_type == CredentialType.AGENT:
        if agent_name:
            print(f"Found agent credentials for '{agent_name}' but SDK credentials are needed.")
        else:
            print("Found agent credentials but SDK credentials are needed.")
        print()
        print("Agent credentials (Ed25519 keys for registered agents) and")
        print("SDK credentials (OAuth tokens for SDK authentication) are different.")
    else:
        print("Found unrecognized credential format.")

    print()
    print("TO FIX:")
    print(f"  1. Download a fresh SDK from: {aim_url}")
    print("  2. Go to Settings → SDK Downloads")
    print("  3. Extract and install: pip install -e aim-sdk-python/")
    print()
    print("Your existing agent credentials are safe and unchanged.")
    print("=" * 72)
    print()


def print_token_expired_error(aim_url: str = "http://localhost:8080") -> None:
    """Print a clear error message when the SDK refresh token is rejected.

    Audit #12: the prior wording told the user to "download a fresh SDK"
    without explaining WHY the local token stopped working — the most
    common cause is that another SDK installation rotated the refresh
    token (server-side rotation invalidates prior copies). This version
    names the cause and surfaces the local path so recovery is obvious.
    """
    print()
    print("=" * 72)
    print("⚠️  SDK REFRESH TOKEN REJECTED")
    print("=" * 72)
    print()
    print(f"The AIM server rejected the refresh token at {SDK_CREDENTIALS_FILE}.")
    print()
    print("WHAT HAPPENED:")
    print("  The SDK rotates its refresh token on every successful refresh")
    print("  and the server only honors the latest one. The token in your")
    print("  local file is no longer the latest, which means one of these")
    print("  happened:")
    print()
    print("  • Another SDK installation (different machine, different venv)")
    print("    refreshed the token and now holds the current one.")
    print("  • A previously bundled SDK download was used elsewhere.")
    print("  • The token was revoked manually from the dashboard.")
    print("  • The refresh token reached its 90-day expiry.")
    print()
    print("TO RECOVER:")
    print(f"  1. Remove the stale file:  rm {SDK_CREDENTIALS_FILE}")
    print(f"  2. Download a fresh SDK from {aim_url} (Settings → SDK Downloads).")
    print("  3. Extract and install: pip install -e aim-sdk-python/")
    print()
    print("Your registered agents are not affected; only the SDK auth")
    print("credential is rotated. Agent Ed25519 keys are separate and remain")
    print("valid.")
    print("=" * 72)
    print()
