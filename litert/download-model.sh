#!/bin/sh
# Downloads the Gemma 4 E2B LiteRT model into the litert-model volume.
# Run once before first start:
#   ./litert/download-model.sh
#
# Or from docker:
#   docker run --rm -v openchore_litert-model:/app/model python:3.12-slim \
#     pip install huggingface_hub && python -c "..."

set -e

MODEL_DIR="${1:-litert-model}"
MODEL_FILE="gemma-4-E2B-it.litertlm"

if [ -f "$MODEL_DIR/$MODEL_FILE" ]; then
    echo "Model already exists at $MODEL_DIR/$MODEL_FILE"
    exit 0
fi

echo "Downloading Gemma 4 E2B LiteRT model (2.58 GB)..."
pip install --quiet huggingface_hub 2>/dev/null || true

python3 -c "
from huggingface_hub import hf_hub_download
hf_hub_download(
    'litert-community/gemma-4-E2B-it-litert-lm',
    'gemma-4-E2B-it.litertlm',
    local_dir='$MODEL_DIR'
)
print('Download complete.')
"
