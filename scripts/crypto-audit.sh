#!/usr/bin/env bash
# Cryptographic Best Practices Audit for AIM
# Checks production code against 11 crypto security requirements.
# Exit codes: 0 = all pass, 1 = one or more failures.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BACKEND_DIR="$REPO_ROOT/apps/backend"
CRYPTO_DIR="$BACKEND_DIR/internal/crypto"
AUTH_DIR="$BACKEND_DIR/internal/infrastructure/auth"

PASS=0
FAIL=0
TOTAL=11

pass() { PASS=$((PASS + 1)); printf "  \033[32m✓\033[0m %s\n" "$1"; }
fail() { FAIL=$((FAIL + 1)); printf "  \033[31m✗\033[0m %s\n" "$1"; }

echo "═══════════════════════════════════════════════════"
echo " AIM Cryptographic Best Practices Audit"
echo "═══════════════════════════════════════════════════"
echo ""

# ---------------------------------------------------------------------------
# 1. No math/rand in crypto paths — must use crypto/rand
# ---------------------------------------------------------------------------
echo "[1/11] Checking for insecure math/rand in crypto code..."
if grep -rn '"math/rand"' "$CRYPTO_DIR" "$AUTH_DIR" 2>/dev/null | grep -v '_test.go' | grep -q .; then
  fail "math/rand found in crypto/auth code (use crypto/rand instead)"
  grep -rn '"math/rand"' "$CRYPTO_DIR" "$AUTH_DIR" 2>/dev/null | grep -v '_test.go'
else
  pass "No math/rand in crypto or auth paths"
fi

# ---------------------------------------------------------------------------
# 2. No weak hash algorithms (MD5, SHA-1, DES, RC4)
# ---------------------------------------------------------------------------
echo "[2/11] Checking for weak hash/cipher algorithms..."
WEAK_PATTERN='crypto/md5\|crypto/des\|crypto/rc4\|crypto/sha1'
if grep -rn "$WEAK_PATTERN" "$BACKEND_DIR/internal/" 2>/dev/null | grep -v '_test.go' | grep -v 'vendor/' | grep -q .; then
  fail "Weak crypto algorithm found in production code"
  grep -rn "$WEAK_PATTERN" "$BACKEND_DIR/internal/" 2>/dev/null | grep -v '_test.go' | grep -v 'vendor/'
else
  pass "No weak hash/cipher algorithms (MD5, SHA-1, DES, RC4)"
fi

# ---------------------------------------------------------------------------
# 3. AES-256 key size enforced (32 bytes)
# ---------------------------------------------------------------------------
echo "[3/11] Checking AES-256 key size enforcement..."
if grep -n 'len(masterKey) != 32' "$CRYPTO_DIR/keyvault.go" >/dev/null 2>&1; then
  pass "AES-256 key size (32 bytes) enforced in keyvault"
else
  fail "Missing AES-256 key size validation in keyvault.go"
fi

# ---------------------------------------------------------------------------
# 4. Ed25519 uses crypto/rand.Reader
# ---------------------------------------------------------------------------
echo "[4/11] Checking Ed25519 uses crypto/rand.Reader..."
if grep -n 'ed25519.GenerateKey(rand.Reader)' "$CRYPTO_DIR/keygen.go" >/dev/null 2>&1; then
  pass "Ed25519 key generation uses rand.Reader"
else
  fail "Ed25519 key generation does not use rand.Reader"
fi

# ---------------------------------------------------------------------------
# 5. PQC (ML-DSA) uses crypto/rand.Reader
# ---------------------------------------------------------------------------
echo "[5/11] Checking PQC key generation uses crypto/rand.Reader..."
PQC_RAND_COUNT=$(grep -c 'rand\.Reader' "$CRYPTO_DIR/pqc/dilithium.go" 2>/dev/null || echo "0")
if [ "$PQC_RAND_COUNT" -ge 3 ]; then
  pass "PQC key generation uses rand.Reader for all ML-DSA modes"
else
  fail "PQC key generation missing rand.Reader usage (found $PQC_RAND_COUNT, expected >= 3)"
fi

# ---------------------------------------------------------------------------
# 6. Only approved PQC algorithms (ML-DSA-44, ML-DSA-65, ML-DSA-87)
# ---------------------------------------------------------------------------
echo "[6/11] Checking for unapproved PQC algorithms..."
APPROVED_PQC='ML-DSA-44\|ML-DSA-65\|ML-DSA-87\|MLDSA44\|MLDSA65\|MLDSA87\|Mode2\|Mode3\|Mode5'
# Look for any dilithium/kyber/sphincs usage that isn't the approved set
UNAPPROVED=$(grep -rn 'kyber\|sphincs\|sike\|bike\|ntru\b' "$CRYPTO_DIR/pqc/" 2>/dev/null | grep -v '_test.go' | grep -v 'vendor/' | grep -vi 'comment\|doc\|readme' || true)
if [ -z "$UNAPPROVED" ]; then
  pass "Only approved PQC algorithms (ML-DSA-44/65/87)"
else
  fail "Unapproved PQC algorithms found"
  echo "$UNAPPROVED"
fi

# ---------------------------------------------------------------------------
# 7. Uses audited circl library for PQC
# ---------------------------------------------------------------------------
echo "[7/11] Checking for audited circl library usage..."
if grep -q 'cloudflare/circl' "$BACKEND_DIR/go.mod" 2>/dev/null; then
  pass "Uses audited cloudflare/circl library for PQC"
else
  fail "Missing cloudflare/circl dependency for PQC"
fi

# ---------------------------------------------------------------------------
# 8. No hardcoded keys or initialization vectors
# ---------------------------------------------------------------------------
echo "[8/11] Checking for hardcoded keys/IVs..."
# Match byte literal arrays that look like keys (16+ hex bytes)
HARDCODED=$(grep -rEn '\[\](byte|uint8)\{(0x[0-9a-fA-F]{2},\s*){15,}' "$BACKEND_DIR/internal/" 2>/dev/null | grep -v '_test.go' | grep -v 'vendor/' || true)
if [ -z "$HARDCODED" ]; then
  pass "No hardcoded keys or initialization vectors"
else
  fail "Potential hardcoded keys/IVs found"
  echo "$HARDCODED"
fi

# ---------------------------------------------------------------------------
# 9. JWT secret minimum 32 characters
# ---------------------------------------------------------------------------
echo "[9/11] Checking JWT secret minimum length..."
if grep -n 'len(secret) < 32' "$AUTH_DIR/jwt.go" >/dev/null 2>&1; then
  pass "JWT secret minimum 32 characters enforced"
else
  fail "Missing JWT secret minimum length validation"
fi

# ---------------------------------------------------------------------------
# 10. bcrypt cost >= 12
# ---------------------------------------------------------------------------
echo "[10/11] Checking bcrypt cost factor..."
BCRYPT_COST=$(grep 'BcryptCost\s*=' "$AUTH_DIR/password.go" 2>/dev/null | sed 's/.*=\s*//' | tr -d '[:space:]' || echo "0")
if [ "$BCRYPT_COST" -ge 12 ]; then
  pass "bcrypt cost factor is $BCRYPT_COST (>= 12)"
else
  fail "bcrypt cost factor is $BCRYPT_COST (must be >= 12)"
fi

# ---------------------------------------------------------------------------
# 11. No TLS 1.0/1.1 or SSL 3.0
# ---------------------------------------------------------------------------
echo "[11/11] Checking for weak TLS/SSL versions..."
WEAK_TLS='tls\.VersionSSL30\|tls\.VersionTLS10\|tls\.VersionTLS11\|MinVersion:\s*0'
if grep -rn "$WEAK_TLS" "$BACKEND_DIR/internal/" 2>/dev/null | grep -v '_test.go' | grep -v 'vendor/' | grep -q .; then
  fail "Weak TLS/SSL version configuration found"
  grep -rn "$WEAK_TLS" "$BACKEND_DIR/internal/" 2>/dev/null | grep -v '_test.go' | grep -v 'vendor/'
else
  pass "No weak TLS/SSL versions (TLS 1.0, 1.1, SSL 3.0)"
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo "═══════════════════════════════════════════════════"
printf " Results: \033[32m%d passed\033[0m, \033[31m%d failed\033[0m out of %d checks\n" "$PASS" "$FAIL" "$TOTAL"
echo "═══════════════════════════════════════════════════"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
