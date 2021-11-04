log_dotter
==============

Used to test the log collection function.

Docker Hub Image
----------------

```shell
docker pull scholarli/log_dotter:latest
```

It can be used directly instead of having to build the image yourself. ([Docker Hub scholarli/log_dotter](https://hub.docker.com/r/scholarli/log_dotter))

Run
---

### Run Binary

```shell
log_dotter -timeout 60 -time 1000
```

### Run Docker Image

```
docker run -p 9094:9094 scholarli/log_dotter -timeout 60 -time 1000
```

Flags
-----

This image is configurable using different flags

| Flag name                    | Default    | Description                                                                                         |
| ---------------------------- | ---------- | --------------------------------------------------------------------------------------------------- |
| time                 | 1000       | log cron time (ms)                                                                                          |
| timeout              | 60         | log cron timeout (minute)                                                                                   |
| port                 | 9094       | Addresses port using of server                                                                              |
| file                 |            | log file path                                                                                               |

API
---

### Metrics

```
http://localhost:9094/metrics
```
