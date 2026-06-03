# L7 Load Balancer

Высокопроизводительный балансировщик нагрузки уровня L7, написанный на чистом Go. Без сторонних фреймворков — только стандартная библиотека.

## Возможности

- **Round Robin балансировка** — распределение трафика между серверами без блокировок (sync/atomic)
- **Active TCP Health Checks** — фоновая проверка доступности серверов каждые 5 секунд
- **Auto Failover** — моментальное исключение упавших узлов из пула
- **Connection Pooling** — переиспользование TCP-соединений через httputil.ReverseProxy
- **Таймауты** — Read/Write/Idle таймауты на все соединения

## Производительность

### Нагрузочное тестирование

Инструмент: `hey`  
Команда: `hey -n 20000 -c 200 http://127.0.0.1:8080`

```text
Summary:
  Total:        1.2342 secs
  Slowest:      0.0847 secs
  Fastest:      0.0002 secs
  Average:      0.0049 secs
  Requests/sec: 16214.67

Status code distribution:
  [200] 20000 responses
Результат: 16 214 RPS (запросов в секунду) при нагрузке 200 конкурентных соединений.
```

Быстрый старт
Запуск балансировщика
```
go run main.go
```
Балансировщик запустится на http://127.0.0.1:8080 и начнет распределять трафик между тремя бэкендами на портах 8081, 8082, 8083.

Проверка работы
```
curl http://127.0.0.1:8080
```
# Backend 1 response
```
curl http://127.0.0.1:8080
```
# Backend 2 response
```
curl http://127.0.0.1:8080
```
# Backend 3 response
Стресс-тест
```
go install github.com/rakyll/hey@latest
hey -n 20000 -c 200 http://127.0.0.1:8080
```
Архитектура
```
Client
  │ (HTTP)
  ▼
Load Balancer :8080
  │    │    │
  ├────┼────┤
  ▼    ▼    ▼
Node1 Node2 Node3
8081  8082  8083
```
Ключевые компоненты
Backend структура
```
type Backend struct {
    url   *url.URL
    alive atomic.Bool
    proxy *httputil.ReverseProxy
}
ServerPool с атомарным счетчиком
Go

type Pool struct {
    backends []*Backend
    current  atomic.Uint32  // Lock-free балансировка
}
Health Check механизм
Go

func (p *Pool) healthCheck() {
    ticker := time.NewTicker(5 * time.Second)
    for range ticker.C {
        for _, b := range p.backends {
            alive := isAlive(b.url)
            b.alive.Store(alive)
        }
    }
}
```
Конфигурация
Отредактируй main.go и измени список серверов:
```
addrs := []string{
    "http://127.0.0.1:8081",
    "http://127.0.0.1:8082",
    "http://127.0.0.1:8083",
}
```
Требования
```
Go 1.20+
```
Технологии
```
Язык: Go
Networking: net, net/http, httputil
Concurrency: goroutines, sync/atomic
I/O: Асинхронное, без блокировок
```
Инженерные решения
1. Атомарный счетчик вместо мьютекса
На критическом пути (балансировка запросов) используется sync/atomic.Uint32 вместо Mutex. Это исключает блокировки и дает максимальную пропускную способность.

2. TCP Health Checks вместо HTTP
Проверка живости происходит на уровне TCP-сокета (быстро), а не через HTTP GET (медленно).

3. Connection Pooling через ReverseProxy
Стандартная библиотека httputil.ReverseProxy управляет пулом соединений и Keep-Alive автоматически.
