# Roadmap web-config-guard

- [x] Сохранить file/directory/stdin/API сценарии; добавить `--fail-on`, YAML/env settings, логи, health, graceful shutdown, лимиты входа.
- [x] Ограничить вложенность, число узлов и YAML aliases; добавить тесты на крупные документы.
- [ ] Усилить обход файлов: безопасная обработка symlink и файловых гонок.
- [ ] Policy-based validation: наборы правил для окружений, управляемое исключение false positives и версия policy.
- [ ] Baseline/diff с устойчивыми fingerprint; затем machine-readable SARIF и GitHub Actions integration.
- [ ] Пользовательские правила после стабилизации schema и оценки безопасности исполнения.
