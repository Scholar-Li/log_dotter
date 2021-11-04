package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/natefinch/lumberjack"
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
	logFile        string
)

func ParseFlags() {
	flag.BoolVar(&logDevelopment, "development", false, "set log development.")
	flag.StringVar(&logLevel, "level", "info", "set log level.")
	flag.Uint64Var(&logTime, "time", 1000, "set log cron time (ms).")
	flag.Uint64Var(&logTimeOut, "timeout", 60, "set log cron timeout (minute).")
	flag.Uint64Var(&serverPort, "port", 9094, "server port.")
	flag.StringVar(&logFile, "file", "", "log file name with path.")
	flag.Parse()
}

var (
	logTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cron_log_total",
		Help: "The total number of cron log",
	})
)

type LogFileCfg struct {
	Filename   string // File Name with path
	MaxSize    int    // Single file size (M)
	MaxBackups int    // Number of backups
	MaxAge     int    // Storage days
	Compress   bool
}

func NewLogger(fileCfg *LogFileCfg, development bool, level string, opts ...zap.Option) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.Development = development

	if err := cfg.Level.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("fail to parse log level, level = %s, err = %s", level, err.Error())
	}

	var l *zap.Logger
	if fileCfg != nil {
		writer := &lumberjack.Logger{
			Filename:   fileCfg.Filename,
			MaxSize:    fileCfg.MaxSize,
			MaxBackups: fileCfg.MaxBackups,
			MaxAge:     fileCfg.MaxAge,
			Compress:   fileCfg.Compress,
		}
		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(cfg.EncoderConfig),
			zapcore.NewMultiWriteSyncer(zapcore.AddSync(writer)),
			cfg.Level,
		)
		l = zap.New(core, append(opts, zap.AddCaller())...)

		// panic info reset to logger file
		// the following info log can't be deleted
		// it assists in creating log files and directories
		// avoid directories that do not have errors
		l.Info("reset panic stderr to logger file, open file again")
		f, err := os.OpenFile(fileCfg.Filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.ModePerm)
		if err != nil {
			return nil, fmt.Errorf("can't open new log file: %v", err)
		}
		defer f.Close()
		err = syscall.Dup2(int(f.Fd()), int(os.Stderr.Fd()))
		if err != nil {
			return nil, fmt.Errorf("failed to redirect stderr to file: %v", err)
		}

		return l, nil
	}

	zap.S().Infof("no logger file, just stdout")
	var e error
	l, e = cfg.Build()
	if e != nil {
		return nil, fmt.Errorf("fail to build logger, err = %s", e.Error())
	}

	if 0 != len(opts) {
		l = l.WithOptions(opts...)
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

	var fileCfg *LogFileCfg

	if logFile != "" {
		fileCfg = &LogFileCfg{
			Filename:   logFile,
			MaxSize:    1024,
			MaxBackups: 1,
			MaxAge:     7,
			Compress:   false,
		}
	}

	l, err := NewLogger(fileCfg, logDevelopment, logLevel)
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
