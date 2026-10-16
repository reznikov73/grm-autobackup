# grm-autobackup — 1С:ГРМ AutoBackup

Трей-приложение для Windows: автоматически скачивает последние бэкапы
приложений 1С:ГРМ (Partner API v2) и хранит их с ограничением по количеству.

## Возможности

- Скачивание последнего готового бэкапа каждого приложения по расписанию:
  «каждые N часов» или «ежедневно в ЧЧ:ММ».
- Докачка: загрузка пишется во временный файл `*.part`; при обрыве связи
  продолжается с места остановки (HTTP Range), после успеха переименовывается
  в финальный файл. Обрывок никогда не «залипает» как готовый бэкап.
- Структура хранения: `<папка>/<Клиент>/<Приложение>/backup_ГГГГ-ММ-ДД_ЧЧ-ММ-СС.zip`
  (имена берутся из API, запрещённые для Windows символы заменяются).
- Лимит бэкапов на приложение — старые удаляются автоматически.
- Исключения приложений по ID и по именам; опция «обрабатывать и остановленные».
- Проверка свободного места перед скачиванием.
- Файловый лог с ротацией: `logs/autobackup.log` (+ `autobackup.log.1`, …).
- Трей: иконка-состояние (зелёная/жёлтая/красная/серая), статус, ручной запуск,
  окна «Настройки», «Монитор действий», «Лог».
- Автозапуск при входе в Windows (запись в HKCU\...\Run, только по галочке в настройках).

## Сборка

```
go build -ldflags "-s -w -H=windowsgui" -o grm-autobackup.exe .
```

Манифест Common Controls v6 уже встроен (`grm-autobackup.manifest` + `rsrc.syso`; пересоздать: `go install github.com/akavel/rsrc@latest` и `rsrc -manifest grm-autobackup.manifest -o rsrc.syso -arch amd64`).

## Запуск

```
.\grm-autobackup.exe
```

Конфигурация ищется рядом с exe: `data/config.json`. Если файла нет — создаётся
со значениями по умолчанию; заполните `api.jwt_token`.

## Конфигурация (data/config.json)

| Поле | Описание |
|---|---|
| `api.url` | Базовый URL Partner API |
| `api.jwt_token` | Bearer-токен |
| `api.timeout_seconds` | Таймаут обычных API-запросов, сек |
| `schedule.enabled` | Включён ли планировщик |
| `schedule.mode` | `interval` — каждые N часов; `daily` — ежедневно в `start_time` |
| `schedule.interval_hours` | N часов для режима interval |
| `schedule.start_time` | Время ЧЧ:ММ для режима daily |
| `storage.path` | Папка хранения бэкапов |
| `storage.backup_limit` | Лимит бэкапов на приложение |
| `exclusions.application_ids` | Исключаемые ID приложений |
| `exclusions.application_names` | Исключаемые имена приложений |
| `include_stopped` | Обрабатывать и остановленные приложения (по умолчанию false) |
| `autostart` | Автозапуск при входе в Windows (по умолчанию false) |
| `logging.level` | debug / info / warn / error |
| `logging.file` | Путь к логу (относительный — от корня проекта) |
| `logging.max_size_mb` / `logging.max_backups` | Ротация лога |

## Служебное

- Тестовый помощник окон: `go build -o uismoke.exe ./cmd/uismoke` →
  `uismoke.exe settings|monitor|log`.
- Тестовый хук: переменная окружения `GRM_TEST_WINDOW=settings|monitor|log`
  открывает окно через 3 секунды после старта (для автотестов).

Документация: `docs/TZ.md`, `docs/API.md`.
