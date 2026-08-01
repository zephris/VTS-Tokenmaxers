# Vendored Source

- Repository: https://github.com/nethical6/conversation-steganography
- Commit: `863a96e77e3c5bb11de3aab55793df58b398f71d`
- Retrieved: 2026-08-01
- License: GPL-3.0; see `LICENSE`

This nested module contains the upstream root Go package and tests plus the Hugging Face and MLX Python model adapters. The upstream CLI is intentionally excluded because the parent server provides the HTTP application boundary.

## Local Adapter Changes

The Hugging Face adapter clones logits before filtering for compatibility with current PyTorch releases and reuses `past_key_values` for sequential prefix requests. Its adapter revision is included in the model fingerprint so participants cannot silently mix cached and uncached implementations.
