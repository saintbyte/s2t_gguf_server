#!/usr/bin/env bash
#
# Скачивает GGUF-модель gigaam-v3-e2e-rnnt (Russian ASR) в каталог ./models
#
# Источник: https://huggingface.co/cstr/gigaam-v3-GGUF
#
# Другие варианты квантизации (замените MODEL ниже):
#   gigaam-v3-e2e-rnnt-f16.gguf   (452 MB, самый точный)
#   gigaam-v3-e2e-rnnt-q8_0.gguf  (249 MB, рекомендуемая точность/размер)
#   gigaam-v3-e2e-rnnt-q4_k.gguf  (154 MB, этот вариант)
#
set -euo pipefail

REPO="${HF_REPO:-cstr/gigaam-v3-GGUF}"
MODEL="${MODEL:-gigaam-v3-e2e-rnnt-q4_k.gguf}"
OUT_DIR="${OUT_DIR:-models}"

URL="${HF_ENDPOINT:-https://huggingface.co}/${REPO}/resolve/main/${MODEL}"

mkdir -p "${OUT_DIR}"

echo "Скачиваю ${URL}"
echo "  -> ${OUT_DIR}/${MODEL}"

if command -v wget >/dev/null 2>&1; then
    wget -c -q --show-progress -O "${OUT_DIR}/${MODEL}" "${URL}"
else
    curl -fL --retry 3 -C - -o "${OUT_DIR}/${MODEL}" "${URL}"
fi

echo "Готово: $(du -h "${OUT_DIR}/${MODEL}" | cut -f1)"
