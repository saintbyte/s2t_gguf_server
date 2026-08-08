# Транскрибация речи (s2t_gguf_server)

Локальный HTTP-сервер для распознавания русской речи на базе GGUF-моделей
(по умолчанию `gigaam-v3-e2e-rnnt-q4_k.gguf`, см. `scripts/download-model.sh`)
через [CrispASR](https://github.com/CrispStrobe/CrispASR). Модель запускается
локально, аудио наружу не отправляется.

## Возможности

- Транскрибация WAV-файлов (загрузка или перетаскивание в окно браузера)
- Запись звука с микрофона прямо в браузере и распознавание её
- Автоматическая передискретизация и понижение до моно (16 kHz)
- Результат с таймкодами сегментов и слов

## Требования

- Go 1.23+
- Модель в формате GGUF (см. ниже)
- Для пересборки библиотек CrispASR: C++17 компилятор, CMake 3.14+
  (опционально CUDA Toolkit / Vulkan SDK — для GPU)

## Установка и запуск

1. Скачайте модель:

   ```sh
   make model
   ```

   По умолчанию качается `gigaam-v3-e2e-rnnt-q4_k.gguf` (154 MB). Другие варианты
   (точность/размер) описаны в `scripts/download-model.sh`.

2. Соберите и запустите:

   ```sh
   make run
   ```

   Или вручную: `go build -o s2t_gguf_server ./cmd/app && ./s2t_gguf_server`.
   Запуск из исходников без сборки: `go run ./cmd/app`

3. Откройте http://localhost:8080

## Makefile

| Цель      | Описание                                              |
|-----------|-------------------------------------------------------|
| `make build` | собрать бинарь в `./s2t_gguf_server`                 |
| `make run`   | скачать модель (если нет), собрать и запустить      |
| `make test`  | прогнать тесты (`go test ./...`)                    |
| `make vet`   | статическая проверка (`go vet ./...`)               |
| `make model` | скачать GGUF-модель (`scripts/download-model.sh`)   |
| `make clean` | удалить собранный бинарь                             |
| `make help`  | показать список целей                                |

Переменные: `GO` (компилятор, по умолчанию `go`), `BINARY` (имя бинаря),
`OUT` (каталог сборки).

## Сборка библиотек CrispASR (third_party)

Библиотеки CrispASR (C++) собираются через CMake. Go-биндинг линкуется
статически против них из `third_party/CrispASR/build_go/` (каталог игнорируется
git, в свежем клоне его нет) — поэтому после пересборки библиотек Go-бинарь
нужно пересобирать заново (`go build`).

> В корне `third_party/CrispASR` лежит `Makefile`, но это сгенерированный
> CMake артефакт с другой машины — пользоваться им нельзя. Рабочий Makefile
> для Go-биндинга — `third_party/CrispASR/bindings/go/Makefile`.

### CPU-сборка

```sh
cd third_party/CrispASR/bindings/go
make whisper          # собирает статические библиотеки в ../build_go
```

Эквивалент вручную через cmake:

```sh
cd third_party/CrispASR
cmake -B build_go -DCMAKE_BUILD_TYPE=Release \
      -DBUILD_SHARED_LIBS=OFF -DCRISPASR_WITH_ESPEAK_NG=OFF \
      -DCRISPASR_MEL_BLAS=OFF -DCRISPASR_OPUS_FETCH=ON
cmake --build build_go --target crispasr-lib -j$(nproc)
```

### Сборка с GPU

Бэкенд выбирается флагом ggml при конфигурации CMake. Можно собрать несколько
бэкендов в один бинарь — ggml сам выберет приоритетный
(CUDA > Metal > Vulkan > MUSA > SYCL > CPU), отдельная настройка не нужна.

| Бэкенд | Флаг | Железо |
|---|---|---|
| CUDA | `-DGGML_CUDA=ON` | NVIDIA (нужен CUDA Toolkit, `nvcc`) |
| Vulkan | `-DGGML_VULKAN=ON` | любое: NVIDIA/AMD/Intel (нужен Vulkan SDK с `glslc`) |
| Metal | `-DGGML_METAL=ON` | Apple Silicon |
| SYCL | `-DGGML_SYCL=ON` | Intel oneAPI |
| MUSA | `-DGGML_MUSA=ON` | Moore Threads |

Максимальная поддержка видеокарт (CUDA + Vulkan в одном бинаре):

```sh
cd third_party/CrispASR
cmake -B build_go -DCMAKE_BUILD_TYPE=Release \
      -DBUILD_SHARED_LIBS=OFF -DCRISPASR_WITH_ESPEAK_NG=OFF \
      -DCRISPASR_MEL_BLAS=OFF -DCRISPASR_OPUS_FETCH=ON \
      -DGGML_CUDA=ON -DGGML_VULKAN=ON \
      -DCMAKE_CUDA_ARCHITECTURES=native
cmake --build build_go --target crispasr-lib -j$(nproc)
```

`CMAKE_CUDA_ARCHITECTURES` можно задать списком (например `80;89;90`), чтобы
бинарь работал на нескольких поколениях NVIDIA-карт, или `native` — только под
установленную карту.

После CUDA-сборки пересоберите сервер с библиотеками CUDA runtime:

```sh
go build -ldflags "-extldflags '-lcudart -lcuda -lcublas'" -o s2t_gguf_server ./cmd/app
```

(Для чистой Vulkan-сборки без CUDA эти `-ldflags` не нужны.)

Полезные переменные окружения при запуске:

- `GGML_CUDA_ENABLE_UNIFIED_MEMORY=1` — вытеснение в RAM при нехватке VRAM
- `CUDA_VISIBLE_DEVICES=0` / `GGML_VK_VISIBLE_DEVICES=0` — выбор конкретной карты

## Конфиг

Настройки читаются из YAML-файла. Поиск по порядку, применяется первый найденный:

1. `/etc/s2t_gguf_server/config.yaml`
2. `./config.yaml` (текущая директория)

Пример — `config.example.yaml`. Пропущенные поля берутся из значений по
умолчанию, флаги командной строки имеют приоритет над конфигом.

## Флаги

| Флаг            | По умолчанию                                   | Описание                     |
|-----------------|------------------------------------------------|------------------------------|
| `-addr`         | `:8080`                                        | адрес HTTP-сервера           |
| `-model`        | `models/gigaam-v3-e2e-rnnt-q4_k.gguf`          | путь к GGUF-модели           |
| `-threads`      | `4`                                            | число потоков ggml           |
| `-max_upload_mb`| `200`                                          | лимит загрузки аудио, МБ     |

## HTTP API

### `POST /api/transcribe`

Принимает аудио WAV одним из способов:

- multipart/form-data с полем `file` (так отправляет веб-интерфейс);
- сырое тело запроса — сам WAV-файл.

Возвращает JSON:

```json
{
  "text": "привет мир",
  "duration": 1.24,
  "segments": [
    {
      "start": 0.0,
      "end": 1.24,
      "text": "привет мир",
      "words": [
        { "text": "привет", "start": 0.0, "end": 0.5, "conf": 0.98 }
      ]
    }
  ]
}
```

Примеры через curl:

```sh
# multipart/form-data (поле file)
curl -F "file=@speech.wav" http://localhost:8080/api/transcribe

# сырое тело WAV
curl --data-binary @speech.wav -H "Content-Type: audio/wav" \
     http://localhost:8080/api/transcribe
```

Лимит загрузки — 200 MB.

### `GET /health`

Возвращает `ok`.

## Структура проекта

```
Makefile                     # сборка/тесты/запуск/модель
config.example.yaml          # пример конфига (копия в /etc/s2t_gguf_server/ или ./)
static/index.html            # веб-интерфейс (загрузка файла + запись с микрофона)
cmd/app/main.go              # HTTP-сервер и обработка транскрибации
cmd/app/config.go            # загрузка настроек из YAML
cmd/app/main_test.go         # тесты (loadWav, HTTP-обработчики)
cmd/app/config_test.go       # тесты загрузки конфига
models/                      # GGUF-модели (скачиваются скриптом)
scripts/download-model.sh    # загрузка модели с Hugging Face
third_party/CrispASR         # форк bindings (подключён через go.mod replace)
```
