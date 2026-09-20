// S5-1: вебхук-sink для проверки end-to-end доставки алертов Alertmanager.
// Принимает webhook-сообщения (Alertmanager webhook_configs), пишет их в
// /var/log/alert-sink.log и отвечает 200. Используется:
//   - локально/в dev-стеке как канал уведомлений (docker-compose.observability.yml);
//   - в CI (docker-compose.alerting-test.yml) как цель теста доставки.
const http = require('http')
const fs = require('fs')

const file = process.env.SINK_LOG_FILE || '/var/log/alert-sink.log'

http
  .createServer((req, res) => {
    let body = ''
    req.setEncoding('utf8')
    req.on('data', (c) => (body += c))
    req.on('end', () => {
      fs.appendFileSync(file, `${new Date().toISOString()} ${req.method} ${req.url}\n${body}\n---\n`)
      res.writeHead(200).end('ok')
    })
  })
  .listen(8099, () => console.log('alert-sink listening on :8099'))