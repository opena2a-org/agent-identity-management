"""
Tests for Capability Auto-Detection
"""

import pytest
import sys
import json
import tempfile
import pathlib
from unittest.mock import patch, mock_open, MagicMock

from aim_sdk.capability_detection import (
    CapabilityDetector,
    auto_detect_capabilities,
    save_capabilities_config
)


class TestCapabilityDetector:
    """Test CapabilityDetector class"""

    def test_init(self):
        """Test detector initialization"""
        detector = CapabilityDetector()
        assert detector.import_to_capability is not None
        assert detector.action_to_capability is not None

    def test_detect_from_imports_with_known_packages(self):
        """Test import detection with known packages"""
        detector = CapabilityDetector()

        # Mock sys.modules with known packages
        with patch.dict(sys.modules, {"requests": MagicMock(), "smtplib": MagicMock()}):
            capabilities = detector.detect_from_imports()

            assert "make_api_calls" in capabilities
            assert "send_email" in capabilities

    def test_detect_from_imports_with_unknown_packages(self):
        """Test import detection with unknown packages"""
        detector = CapabilityDetector()

        # Mock sys.modules with ONLY unknown packages (clear real imports)
        with patch.dict(sys.modules, {"unknown_package": MagicMock()}, clear=True):
            capabilities = detector.detect_from_imports()

            # Should not detect unknown packages
            assert len(capabilities) == 0

    def test_detect_from_config_file_exists(self):
        """Test config file detection when file exists"""
        detector = CapabilityDetector()

        config_data = {
            "capabilities": ["custom_capability_1", "custom_capability_2"],
            "last_updated": "2025-10-09T12:00:00Z"
        }

        with tempfile.TemporaryDirectory() as tmpdir:
            config_path = pathlib.Path(tmpdir) / "capabilities.json"
            with open(config_path, 'w') as f:
                json.dump(config_data, f)

            # Mock _get_capabilities_config_path to return our temp file
            with patch.object(detector, '_get_capabilities_config_path', return_value=config_path):
                capabilities = detector.detect_from_config()

                assert "custom_capability_1" in capabilities
                assert "custom_capability_2" in capabilities

    def test_detect_from_config_file_not_exists(self):
        """Test config file detection when file doesn't exist"""
        detector = CapabilityDetector()

        # Mock _get_capabilities_config_path to return None
        with patch.object(detector, '_get_capabilities_config_path', return_value=None):
            capabilities = detector.detect_from_config()

            assert len(capabilities) == 0

    def test_detect_from_config_invalid_json(self):
        """Test config file detection with invalid JSON"""
        detector = CapabilityDetector()

        with tempfile.TemporaryDirectory() as tmpdir:
            config_path = pathlib.Path(tmpdir) / "capabilities.json"
            with open(config_path, 'w') as f:
                f.write("invalid json{}")

            # Mock _get_capabilities_config_path to return our temp file
            with patch.object(detector, '_get_capabilities_config_path', return_value=config_path):
                capabilities = detector.detect_from_config()

                # Should return empty list on parse error
                assert len(capabilities) == 0

    def test_detect_all_combines_sources(self):
        """Test that detect_all combines all detection methods"""
        detector = CapabilityDetector()

        # Mock different detection methods
        with patch.object(detector, 'detect_from_imports', return_value=["import_cap_1"]):
            with patch.object(detector, 'detect_from_config', return_value=["config_cap_1"]):
                with patch.object(detector, 'detect_from_decorators', return_value=["decorator_cap_1"]):
                    capabilities = detector.detect_all()

                    assert "import_cap_1" in capabilities
                    assert "config_cap_1" in capabilities
                    assert "decorator_cap_1" in capabilities
                    assert len(capabilities) == 3

    def test_detect_all_removes_duplicates(self):
        """Test that detect_all removes duplicate capabilities"""
        detector = CapabilityDetector()

        # Mock detection methods to return overlapping capabilities
        with patch.object(detector, 'detect_from_imports', return_value=["capability_1", "capability_2"]):
            with patch.object(detector, 'detect_from_config', return_value=["capability_2", "capability_3"]):
                with patch.object(detector, 'detect_from_decorators', return_value=[]):
                    capabilities = detector.detect_all()

                    # Should have unique capabilities only
                    assert capabilities.count("capability_1") == 1
                    assert capabilities.count("capability_2") == 1
                    assert capabilities.count("capability_3") == 1
                    assert len(capabilities) == 3

    def test_detect_all_sorts_capabilities(self):
        """Test that detect_all returns sorted capabilities"""
        detector = CapabilityDetector()

        with patch.object(detector, 'detect_from_imports', return_value=["zulu", "alpha"]):
            with patch.object(detector, 'detect_from_config', return_value=["bravo"]):
                with patch.object(detector, 'detect_from_decorators', return_value=[]):
                    capabilities = detector.detect_all()

                    # Should be sorted alphabetically
                    assert capabilities == ["alpha", "bravo", "zulu"]

    def test_import_to_capability_mappings(self):
        """Test known import-to-capability mappings"""
        detector = CapabilityDetector()

        # Test common packages
        assert detector.import_to_capability["requests"] == "make_api_calls"
        assert detector.import_to_capability["smtplib"] == "send_email"
        assert detector.import_to_capability["psycopg2"] == "access_database"
        assert detector.import_to_capability["subprocess"] == "execute_code"

    def test_action_to_capability_mappings(self):
        """Test action-to-capability mappings"""
        detector = CapabilityDetector()

        # Test common actions
        assert detector.action_to_capability["read_database"] == "access_database"
        assert detector.action_to_capability["send_email"] == "send_email"
        assert detector.action_to_capability["execute_command"] == "execute_code"


class TestAutoDetectCapabilities:
    """Test auto_detect_capabilities convenience function"""

    def test_auto_detect_capabilities(self):
        """Test auto_detect_capabilities function"""
        # Mock sys.modules with known packages
        with patch.dict(sys.modules, {"requests": MagicMock()}):
            capabilities = auto_detect_capabilities()

            assert isinstance(capabilities, list)
            assert "make_api_calls" in capabilities


class TestSaveCapabilitiesConfig:
    """Test save_capabilities_config function"""

    def test_save_capabilities_config(self):
        """Test saving capabilities to config file"""
        capabilities = ["capability_1", "capability_2"]

        with tempfile.TemporaryDirectory() as tmpdir:
            aim_dir = pathlib.Path(tmpdir) / ".aim"

            # Mock Path.home() to return our temp directory
            with patch('pathlib.Path.home', return_value=pathlib.Path(tmpdir)):
                save_capabilities_config(capabilities)

                # Verify file was created
                config_path = aim_dir / "capabilities.json"
                assert config_path.exists()

                # Verify content
                with open(config_path, 'r') as f:
                    config = json.load(f)

                assert config["capabilities"] == capabilities
                assert "last_updated" in config
                assert config["version"] == "1.0.0"


if __name__ == '__main__':
    pytest.main([__file__, '-v'])
