log_dotter
==============

Used to test the log collection function. Generate 15w logs at a time.

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
docker run -p 9093:9093 -p 9094:9094 scholarli/log_dotter
```

Flags
-----

This image is configurable using different flags

| Flag name                    | Default    | Description                                                                                         |
| ---------------------------- | ---------- | --------------------------------------------------------------------------------------------------- |
| http                         | false      | start http                                                                         |
| interval                     | 1000       | log cron interval time (s)                                                                         |
| timeout                      | 60         | log cron timeout (minute)                                                                           |
| port                         | 9094       | Addresses port using of server                                                                      |
| file                         |            | log file path                                                                                       |

API
---

### Metrics

```
http://localhost:9094/metrics
```

### operate

#### Reset
```shell
curl --location --request POST '127.0.0.1:9093/reset' \
--header 'Content-Type: application/json' \
--data-raw '{
    "timeout": 60,
    "interval": 1
}'
```

#### Get
```shell
curl --location --request GET '127.0.0.1:9093/config'
```

#### Stop
```shell
curl --location --request POST '127.0.0.1:9093/stop'
```