package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const server = "log_dotter"

var (
	logDevelopment bool
	logLevel       string
	logTime        uint64
	logTimeOut     uint64
	serverPort     uint64
)

func ParseFlags() {
	flag.BoolVar(&logDevelopment, "development", false, "set log development.")
	flag.StringVar(&logLevel, "level", "info", "set log level.")
	flag.Uint64Var(&logTime, "time", 1000, "set log cron time (ms).")
	flag.Uint64Var(&logTimeOut, "timeout", 60, "set log cron timeout (minute).")
	flag.Uint64Var(&serverPort, "port", 9094, "server port.")
	flag.Parse()
}

var (
	logTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cron_log_total",
		Help: "The total number of cron log",
	})
)

func NewLogger(development bool, level string) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.Development = development

	if err := cfg.Level.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("fail to parse log level, level = %s, err = %s", level, err.Error())
	}

	l, e := cfg.Build()
	if e != nil {
		return nil, fmt.Errorf("fail to build logger, err = %s", e.Error())
	}

	return l, nil
}

func CronLog(ctx context.Context, d time.Duration)  {
	tick := time.Tick(d)

	subCtx, _ := context.WithCancel(ctx)
	for {
		select {
		case <-subCtx.Done():
			zap.L().Info("server is stopped with timeout.")
			return
		case <-tick:
		}

		// Random generation was not used
		// because avoiding the generation of random data might affect the actual time of logging
		zap.L().Info("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
		logTotal.Inc()
	}
}

func main() {
	ParseFlags()

	l, err := NewLogger(logDevelopment, logLevel)
	if err != nil {
		panic(err)
	}

	l.With(zap.String("name", server))
	zap.ReplaceGlobals(l)

	l.Info("server is started.")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute * time.Duration(logTimeOut))

	go CronLog(ctx, time.Millisecond * time.Duration(logTime))

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		err = http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", 9094), nil)
		if err != nil {
			l.Panic("failed to listen and serve admin http server.", zap.Error(err))
			cancel()
		}
	}()

	select {
	case <-ctx.Done():
		os.Exit(0)
		return
	}
}
