"""Shared constants for crypto golden-vector generation and verification.

The test password lives only in process memory for KDF derivation. It is never
written into on-disk vector JSON (CodeQL: clear-text storage of sensitive data).
"""

# Fixed passphrase used to produce committed golden vectors under tests/vectors/.
# Must stay stable or all vector files need regeneration via generate_vectors.py.
VECTOR_TEST_PASSWORD = "vector-test-password!"
