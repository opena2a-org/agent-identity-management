"""
AIM Python SDK Setup
"""

from setuptools import setup, find_packages
import os

# Read version from VERSION file (single source of truth)
version_file = os.path.join(os.path.dirname(__file__), "VERSION")
with open(version_file, "r", encoding="utf-8") as vf:
    version = vf.read().strip()

with open("README.md", "r", encoding="utf-8") as fh:
    long_description = fh.read()

setup(
    name="aim-sdk",
    version=version,
    author="OpenA2A",
    author_email="info@opena2a.org",
    description="Python SDK for AIM (Agent Identity Management) - Automatic identity verification for AI agents",
    long_description=long_description,
    long_description_content_type="text/markdown",
    url="https://github.com/opena2a-org/agent-identity-management",
    packages=find_packages(),
    classifiers=[
        "Development Status :: 5 - Production/Stable",
        "Intended Audience :: Developers",
        "Topic :: Software Development :: Libraries :: Python Modules",
        "Topic :: Security :: Cryptography",
        "License :: OSI Approved :: GNU Affero General Public License v3",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
        "Programming Language :: Python :: 3.12",
    ],
    python_requires=">=3.8",
    install_requires=[
        "requests>=2.28.0",
        "PyNaCl>=1.5.0",  # Ed25519 signing
        "cryptography>=43.0.1",  # REQUIRED: Secure credential encryption (CVE-2023-50782 fix)
        "keyring>=24.0.0",  # REQUIRED: System keyring for encryption keys
    ],
    extras_require={
        "dev": [
            "pytest>=7.0.0",
            "pytest-cov>=4.0.0",
            "pytest-mock>=3.10.0",
            "black>=24.3.0",  # CVE-2024-48 fix
            "flake8>=6.0.0",
            "mypy>=1.0.0",
        ],
        "mcp": [
            "mcp>=1.23.0",  # Official MCP SDK (CVE-2025-53366, CVE-2025-66416 fix)
            "anyio>=4.0.0",  # Async runtime for MCP client
        ],
        "rich": [
            "rich>=13.0.0",  # Beautiful terminal output (Stripe-style formatting)
        ]
    },
    keywords="aim agent identity management verification security cryptography ed25519",
    project_urls={
        "Bug Reports": "https://github.com/opena2a-org/agent-identity-management/issues",
        "Source": "https://github.com/opena2a-org/agent-identity-management",
        "Documentation": "https://opena2a.org/docs/aim",
    },
)
