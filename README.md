# L7 Load Balancer

Высокопроизводительный балансировщик нагрузки уровня L7, написанный на чистом Go. Без сторонних фреймворков — только стандартная библиотека.

## Возможности

- **Round Robin балансировка** — распределение трафика между серверами без блокировок (sync/atomic)
- **Active TCP Health Checks** — фоновая проверка доступности серверов каждые 5 секунд
- **Auto Failover** — моментальное исключение упавших узлов из пула
- **Connection Pooling** — переиспользование TCP-соединений через httputil.ReverseProxy
- **Таймауты** — Read/Write/Idle таймауты на все соединения

## Быстрый старт

```bash
go run main.go backends.go
```
Балансировщик запустится на http://127.0.0.1:8080 и начнет распределять трафик между тремя бэкендами.

Проверка работы
```
curl http://127.0.0.1:8080
```
# backend port 8081
```
curl http://127.0.0.1:8080
```
# backend port 8082
```
curl http://127.0.0.1:8080
```
# backend port 8083
При каждом запросе порт меняется по кругу.

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
Файлы проекта
```
├── main.go       — Балансировщик, healthCheck, Round Robin логика
└── backends.go   — Три тестовых HTTP-сервера на портах 8081-8083
```
Инженерные решения
1. Атомарный счетчик вместо мьютекса
На критическом пути (балансировка запросов) используется sync/atomic.Uint32 вместо Mutex. Это исключает блокировки и дает максимальную пропускную способность.

2. TCP Health Checks вместо HTTP
Проверка живости происходит на уровне TCP-сокета (быстро), а не через HTTP GET (медленно).

3. Connection Pooling через ReverseProxy
Стандартная библиотека httputil.ReverseProxy управляет пулом соединений и Keep-Alive автоматически.

4. Auto Failover без рестарта
Если сервер падает, Health Check автоматически помечает его как DOWN. Следующий запрос уйдет на живой сервер. Пользователь ничего не заметит.

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
