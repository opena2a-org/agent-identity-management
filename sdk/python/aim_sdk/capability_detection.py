"""
AIM SDK - Agent Capability Auto-Detection

This module automatically detects agent capabilities through various methods:

1. Decorator Analysis - Scans @agent.perform_action() decorator calls
2. Import Analysis - Infers capabilities from imported packages
3. Config File - Reads explicit declarations from .aim/capabilities.json

Detection results can be used during agent registration or reported separately.
"""

import ast
import os
import sys
import json
import pathlib
import inspect
from typing import List, Set, Optional, Dict, Any
from datetime import datetime, timezone

__version__ = "1.0.0"


class CapabilityDetector:
    """
    Auto-detector for agent capabilities.

    This class scans the Python environment to automatically detect
    what capabilities an agent has based on code analysis, imports,
    and configuration files.

    Example:
        from aim_sdk import CapabilityDetector

        detector = CapabilityDetector()
        capabilities = detector.detect_all()
        print(f"Detected capabilities: {capabilities}")
    """

    def __init__(self):
        """Initialize the capability detector."""
        # Map common Python packages to capabilities
        self.import_to_capability = {
            # File System
            "os": "read_files",
            "shutil": "write_files",
            "pathlib": "read_files",

            # Email
            "smtplib": "send_email",
            "email": "send_email",
            "imaplib": "read_email",

            # Database
            "psycopg2": "access_database",
            "pymongo": "access_database",
            "mysql": "access_database",
            "sqlite3": "access_database",
            "sqlalchemy": "access_database",

            # HTTP/API
            "requests": "make_api_calls",
            "urllib": "make_api_calls",
            "aiohttp": "make_api_calls",
            "httpx": "make_api_calls",

            # Code Execution
            "subprocess": "execute_code",
            "exec": "execute_code",
            "eval": "execute_code",

            # Cloud Services
            "boto3": "access_cloud_services",
            "google.cloud": "access_cloud_services",
            "azure": "access_cloud_services",

            # Web Scraping
            "beautifulsoup4": "web_scraping",
            "bs4": "web_scraping",
            "scrapy": "web_scraping",
            "selenium": "web_automation",
            "playwright": "web_automation",

            # Data Processing
            "pandas": "data_processing",
            "numpy": "data_processing",

            # AI/ML
            "openai": "ai_model_access",
            "anthropic": "ai_model_access",
            "langchain": "ai_agent_framework",
            "crewai": "ai_agent_framework",

            # File Operations
            "json": "read_files",
            "yaml": "read_files",
            "csv": "read_files",
            "pickle": "read_files",
        }

        # Common action patterns to capability mapping
        self.action_to_capability = {
            "read_database": "access_database",
            "write_database": "access_database",
            "query_database": "access_database",
            "send_email": "send_email",
            "read_email": "read_email",
            "read_file": "read_files",
            "write_file": "write_files",
            "delete_file": "write_files",
            "execute_command": "execute_code",
            "run_code": "execute_code",
            "make_request": "make_api_calls",
            "call_api": "make_api_calls",
            "web_search": "web_scraping",
            "browse_web": "web_automation",
        }

    def detect_all(self) -> List[str]:
        """
        Run all detection methods and return combined unique capabilities.

        Returns:
            List of unique capability strings
        """
        capabilities: Set[str] = set()

        # 1. Detect from imports
        import_caps = self.detect_from_imports()
        capabilities.update(import_caps)

        # 2. Detect from config file
        config_caps = self.detect_from_config()
        capabilities.update(config_caps)

        # 3. Detect from decorators (if called from within agent code)
        try:
            decorator_caps = self.detect_from_decorators()
            capabilities.update(decorator_caps)
        except Exception:
            # Decorator scanning might fail in some environments
            pass

        return sorted(list(capabilities))

    def detect_from_imports(self) -> List[str]:
        """
        Detect capabilities from Python imports.

        Scans sys.modules for imported packages and infers capabilities
        based on known package-to-capability mappings.

        Returns:
            List of detected capabilities
        """
        capabilities: Set[str] = set()

        # Scan currently loaded modules
        for module_name in sys.modules.keys():
            # Extract top-level package name
            top_level = module_name.split('.')[0]

            # Check if this package maps to a capability
            if top_level in self.import_to_capability:
                capability = self.import_to_capability[top_level]
                capabilities.add(capability)

        return list(capabilities)

    def detect_from_config(self) -> List[str]:
        """
        Detect capabilities from .aim/capabilities.json config file.

        Reads explicit capability declarations from user's config file.

        Returns:
            List of configured capabilities
        """
        config_path = self._get_capabilities_config_path()

        if not config_path or not config_path.exists():
            return []

        try:
            with open(config_path, 'r') as f:
                config = json.load(f)

            # Extract capabilities list
            capabilities = config.get("capabilities", [])
            if isinstance(capabilities, list):
                return capabilities

        except Exception:
            # Silently fail - don't break agent execution
            pass

        return []

    def detect_from_decorators(self) -> List[str]:
        """
        Detect capabilities from @agent.perform_action() decorator usage.

        Scans the calling module's source code for decorator patterns
        and extracts action types to infer capabilities.

        Returns:
            List of detected capabilities from decorators
        """
        capabilities: Set[str] = set()

        try:
            # Get the calling module's frame
            frame = inspect.currentframe()
            if not frame:
                return []

            # Navigate up the call stack to find the user's module
            caller_frame = frame.f_back
            while caller_frame:
                caller_module = inspect.getmodule(caller_frame)
                if caller_module and hasattr(caller_module, '__file__'):
                    source_file = caller_module.__file__
                    if source_file and not source_file.endswith('aim_sdk'):
                        # Found user module - scan its source
                        caps = self._scan_file_for_decorators(source_file)
                        capabilities.update(caps)
                        break
                caller_frame = caller_frame.f_back

        except Exception:
            # AST parsing might fail - that's okay
            pass

        return list(capabilities)

    def _scan_file_for_decorators(self, file_path: str) -> Set[str]:
        """
        Scan a Python file for @agent.perform_action() decorators.

        Uses AST parsing to find decorator calls and extract action types.

        Args:
            file_path: Path to Python source file

        Returns:
            Set of capabilities detected from decorators
        """
        capabilities: Set[str] = set()

        try:
            with open(file_path, 'r') as f:
                source_code = f.read()

            # Parse AST
            tree = ast.parse(source_code)

            # Find all function definitions with decorators
            for node in ast.walk(tree):
                if isinstance(node, ast.FunctionDef):
                    for decorator in node.decorator_list:
                        # Check if decorator is agent.perform_action(...)
                        if self._is_perform_action_decorator(decorator):
                            action_type = self._extract_action_type(decorator)
                            if action_type:
                                # Map action type to capability
                                capability = self.action_to_capability.get(
                                    action_type,
                                    action_type  # Use action_type as capability if no mapping
                                )
                                capabilities.add(capability)

        except Exception:
            # AST parsing can fail - that's okay
            pass

        return capabilities

    def _is_perform_action_decorator(self, decorator: ast.AST) -> bool:
        """Check if decorator is @agent.perform_action or @client.perform_action"""
        if isinstance(decorator, ast.Call):
            func = decorator.func
            if isinstance(func, ast.Attribute):
                return func.attr == "perform_action"
        return False

    def _extract_action_type(self, decorator: ast.Call) -> Optional[str]:
        """Extract action_type argument from @agent.perform_action() call"""
        # Check positional arguments
        if decorator.args and len(decorator.args) > 0:
            arg = decorator.args[0]
            if isinstance(arg, ast.Constant):
                return arg.value

        # Check keyword arguments
        for keyword in decorator.keywords:
            if keyword.arg == "action_type":
                if isinstance(keyword.value, ast.Constant):
                    return keyword.value.value

        return None

    def _get_capabilities_config_path(self) -> Optional[pathlib.Path]:
        """Get path to .aim/capabilities.json config file

        Always uses home directory (~/.aim/) for config - never project directory.
        This ensures configs are user-specific and not accidentally committed to version control.
        """
        home = pathlib.Path.home()
        config_path = home / ".aim" / "capabilities.json"
        return config_path if config_path.exists() else None


def auto_detect_capabilities() -> List[str]:
    """
    Convenience function for quick capability detection.

    This is a helper function that creates a CapabilityDetector
    and runs all detection methods.

    Returns:
        List of detected capabilities

    Example:
        from aim_sdk import auto_detect_capabilities

        capabilities = auto_detect_capabilities()
        print(f"Your agent has these capabilities: {capabilities}")
    """
    detector = CapabilityDetector()
    return detector.detect_all()


def save_capabilities_config(capabilities: List[str]) -> None:
    """
    Save capabilities to .aim/capabilities.json config file.

    This allows explicit declaration of capabilities that may not
    be auto-detectable.

    Args:
        capabilities: List of capability strings to save

    Example:
        save_capabilities_config([
            "read_files",
            "write_files",
            "send_email",
            "access_database"
        ])
    """
    home = pathlib.Path.home()
    aim_dir = home / ".aim"
    aim_dir.mkdir(exist_ok=True)

    config_path = aim_dir / "capabilities.json"

    config = {
        "capabilities": capabilities,
        "last_updated": datetime.now(timezone.utc).isoformat(),
        "version": "1.0.0"
    }

    with open(config_path, 'w') as f:
        json.dump(config, f, indent=2)

    os.chmod(config_path, 0o600)  # Secure permissions
