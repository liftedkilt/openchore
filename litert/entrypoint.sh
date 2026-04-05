#!/bin/sh
set -e

MODEL_DIR=$(dirname "$MODEL_PATH")
mkdir -p "$MODEL_DIR"

if [ ! -f "$MODEL_PATH" ]; then
    echo "Model not found at $MODEL_PATH — downloading from HuggingFace..."
    echo "This is a one-time download (~3.7 GB). The model is saved to a persistent volume."
    python -c "
from huggingface_hub import hf_hub_download
import os
hf_hub_download(
    os.environ['MODEL_REPO'],
    os.environ['MODEL_FILE'],
    local_dir=os.path.dirname(os.environ['MODEL_PATH']),
)
print('Download complete.')
"
else
    echo "Model found at $MODEL_PATH"
fi

exec python server.py
