---
title: ESL 控制 API
order: 2
---

# ESL 控制 API

## 应用 FreeSWITCH 事件

```http
POST /esl/events/apply
Content-Type: application/json

{
  "eventId": "evt-1",
  "eventName": "CHANNEL_CREATE",
  "callId": "call-1",
  "uuid": "uuid-1",
  "fsAddr": "192.168.107.6:8021",
  "legRole": "customer"
}
```

## 控制命令

| API | 命令 |
| --- | --- |
| `/esl/control/playback` | 播放音频 |
| `/esl/control/break` | 停止播放 |
| `/esl/control/bridge` | 桥接 |
| `/esl/control/hangup` | 挂断 |
| `/esl/control/audio-stream` | 音频流 |
