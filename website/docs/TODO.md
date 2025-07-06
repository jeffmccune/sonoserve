---
sidebar_position: 4
---

# TODO

Technical debt items to address in the future.

## Build System

- [ ] Fix Makefile warnings: "warning: overriding commands for target 'build'" - These warnings are technical debt but not important now

## Get Speaker Name Panic

- [x] Fix eventSub panic - FIXED

When discovering speakers, there is a panic that needs to be fixed.  The refresh speakers button fails to fetch when this panic is triggered.

Error:

```
2025/07/06 14:53:07 Getting room name for Sonos device at 192.168.4.129
2025/07/06 14:53:07 http: panic serving [::1]:61624: pattern "/eventSub" (registered at /Users/jeff/go/pkg/mod/github.com/ianr0bkny/go-sonos@v0.0.0-20171025003233-056585059953/upnp/event.go:149) conflicts with pattern "/eventSub" (registered at /Users/jeff/go/pkg/mod/github.com/ianr0bkny/go-sonos@v0.0.0-20171025003233-056585059953/upnp/event.go:149):
/eventSub matches the same requests as /eventSub
goroutine 32 [running]:
net/http.(*conn).serve.func1()
        /opt/homebrew/Cellar/go/1.24.2/libexec/src/net/http/server.go:1947 +0xb0
panic({0x10496fc00?, 0x140004301c0?})
        /opt/homebrew/Cellar/go/1.24.2/libexec/src/runtime/panic.go:792 +0x124
net/http.(*ServeMux).register(...)
        /opt/homebrew/Cellar/go/1.24.2/libexec/src/net/http/server.go:2872
net/http.Handle({0x1045470a5?, 0x104928550?}, {0x1049e2da0?, 0x14000200d80?})
        /opt/homebrew/Cellar/go/1.24.2/libexec/src/net/http/server.go:2856 +0x94
github.com/ianr0bkny/go-sonos/upnp.(*upnpDefaultReactor).Init(0x14000200d80, {0x10454564e, 0x3}, {0x104545400, 0x1})
        /Users/jeff/go/pkg/mod/github.com/ianr0bkny/go-sonos@v0.0.0-20171025003233-056585059953/upnp/event.go:149 +0x24c
github.com/ianr0bkny/go-sonos.MakeReactor({0x10454564e, 0x3}, {0x104545400, 0x1})
        /Users/jeff/go/pkg/mod/github.com/ianr0bkny/go-sonos@v0.0.0-20171025003233-056585059953/sonos.go:241 +0x138
main.getSonosRoomName({0x14000403707, 0xd})
        /Users/jeff/esp/sonoserve/main.go:205 +0x184
main.discoverSonosDevices()
        /Users/jeff/esp/sonoserve/main.go:163 +0x64c
main.discoverHandler({0x1049e5408, 0x1400055a0e0}, 0x140000aca01?)
        /Users/jeff/esp/sonoserve/main.go:268 +0xcc
net/http.HandlerFunc.ServeHTTP(0x14000130180?, {0x1049e5408?, 0x1400055a0e0?}, 0x1c?)
        /opt/homebrew/Cellar/go/1.24.2/libexec/src/net/http/server.go:2294 +0x38
net/http.(*ServeMux).ServeHTTP(0x14000551140?, {0x1049e5408, 0x1400055a0e0}, 0x14000436500)
        /opt/homebrew/Cellar/go/1.24.2/libexec/src/net/http/server.go:2822 +0x1b4
main.main.corsMiddleware.func6({0x1049e5408, 0x1400055a0e0}, 0x14000436500)
        /Users/jeff/esp/sonoserve/main.go:61 +0x130
net/http.HandlerFunc.ServeHTTP(0x140002aa820?, {0x1049e5408?, 0x1400055a0e0?}, 0x140000acb60?)
        /opt/homebrew/Cellar/go/1.24.2/libexec/src/net/http/server.go:2294 +0x38
net/http.serverHandler.ServeHTTP({0x140005510b0?}, {0x1049e5408?, 0x1400055a0e0?}, 0x6?)
        /opt/homebrew/Cellar/go/1.24.2/libexec/src/net/http/server.go:3301 +0xbc
net/http.(*conn).serve(0x1400014e510, {0x1049e59a0, 0x140000bc240})
        /opt/homebrew/Cellar/go/1.24.2/libexec/src/net/http/server.go:2102 +0x52c
created by net/http.(*Server).Serve in goroutine 5
        /opt/homebrew/Cellar/go/1.24.2/libexec/src/net/http/server.go:3454 +0x3d8
```
