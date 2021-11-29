package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/natefinch/lumberjack"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const server = "log_dotter"

type Dotter struct {
	ctx    context.Context
	cancel context.CancelFunc
	t      <-chan time.Time

	Timeout  uint64 `json:"timeout"`  // set log cron timeout (minute)
	Interval uint64 `json:"interval"` // set log cron interval time (ms)
}

func (d *Dotter) Restart(ctx context.Context, interval, timeout uint64) {
	d.ctx, d.cancel = context.WithTimeout(ctx, time.Minute*time.Duration(timeout))
	d.t = time.Tick(time.Duration(interval) * time.Second)

	d.Interval = interval
	d.Timeout = timeout

	go d.startCron()
}

func (d *Dotter) Tick() <-chan time.Time {
	return d.t
}

func (d *Dotter) startCron() {
	zap.L().Info("dotter server is started.")
	l := zap.L()
	for {
		select {
		case <-d.ctx.Done():
			zap.L().Info("dotter server is stopped with timeout.")
			return
		case <-d.Tick():
		}

		// max 170*1000
		for i := 0; i < 150000; i++ {
			// Random generation was not used
			// because avoiding the generation of random data might affect the actual time of logging
			l.Info("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
			logTotal.Inc()
		}
	}
}

var dotter = Dotter{}

type LogFileCfg struct {
	Filename   string // File Name with path
	MaxSize    int    // Single file size (M)
	MaxBackups int    // Number of backups
	MaxAge     int    // Storage days
	Compress   bool
}

type LogWriter struct {
	*zap.SugaredLogger
}

func (l *LogWriter) Write(d []byte) (n int, err error) {

	l.Info(string(d))
	return len(d), nil
}

type NullWriter struct{}

func (l *NullWriter) Write(d []byte) (n int, err error) {
	return len(d), nil
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

func NewGinEngine(out io.Writer, logLevel string) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	e := gin.New()
	if logLevel == "debug" {
		gin.SetMode(gin.DebugMode)
		out = os.Stdout
	}

	e.Use(gin.RecoveryWithWriter(out))
	e.Use(gin.LoggerWithWriter(out))
	e.Use(CheckParamsIsValid())

	return e
}

func CheckParamsIsValid() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		paramsMap := make(map[string]string)
		for _, param := range ctx.Params {
			paramsMap[param.Key] = param.Value
			if param.Value == "" {
				ctx.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("params[%s] is required!", param.Key)})
				ctx.Abort()
				return
			}

			//if strings.Contains(param.Value, " ") {
			//	ctx.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("[%s] cannot contain spaces!", param.Key)})
			//	ctx.Abort()
			//	return
			//}
		}
	}
}

var (
	httpEnable     bool
	logDevelopment bool
	logLevel       string
	logInterval    uint64
	logTimeOut     uint64
	logFile        string
)

func ParseFlags() {
	flag.BoolVar(&httpEnable, "http", false, "start http.")
	flag.BoolVar(&logDevelopment, "development", false, "set log development.")
	flag.StringVar(&logLevel, "level", "info", "set log level.")
	flag.Uint64Var(&logInterval, "interval", 1000, "set log cron interval time (s).")
	flag.Uint64Var(&logTimeOut, "timeout", 60, "set log cron timeout (minute).")
	flag.StringVar(&logFile, "file", "", "log file name with path.")
	flag.Parse()
}

var (
	logTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cron_log_total",
		Help: "The total number of cron log",
	})
)

func SyncStartWithHTTP(ctx context.Context, cancel context.CancelFunc) {
	mainCtx := ctx

	engine := NewGinEngine(&NullWriter{}, logLevel)
	//engine := NewGinEngine(&LogWriter{zap.S()}, logLevel)
	//engine := gin.Default()

	engine.GET("/config", func(ctx *gin.Context) {
		if dotter.ctx == nil {
			ctx.JSON(http.StatusOK, gin.H{
				"deadline": "",
				"interval": "",
			})
			return
		}
		deadline, _ := dotter.ctx.Deadline()
		ctx.JSON(http.StatusOK, gin.H{
			"deadline": deadline,
			"interval": dotter.Interval,
		})
	})

	engine.POST("/reset", func(ctx *gin.Context) {
		req := Dotter{}
		err := ctx.BindJSON(&req)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}

		if req.Timeout == 0 || req.Interval == 0 {
			ctx.JSON(http.StatusBadRequest, gin.H{"message": "timeout, interval is required"})
			return
		}

		dotter.Restart(mainCtx, req.Interval, req.Timeout)
		logTotal.Set(0)
		ctx.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	engine.POST("/stop", func(ctx *gin.Context) {
		dotter.cancel()
		ctx.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	go func() {
		err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", 9093), engine)
		if err != nil {
			zap.S().Panic("failed to listen and serve http server.", zap.Error(err))
			cancel()
		}
	}()
}

func SyncStartWithShellTimeOut(ctx context.Context, cancel context.CancelFunc, interval, timeout uint64) {
	go func() {
		dotter.Restart(ctx, interval, timeout)
		time.Sleep(time.Minute * time.Duration(dotter.Timeout))
		cancel()
	}()
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

	ctx, cancel := context.WithCancel(context.Background())

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		err = http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", 9094), nil)
		if err != nil {
			l.Panic("failed to listen and serve admin http server.", zap.Error(err))
			cancel()
		}
	}()

	if httpEnable {
		SyncStartWithHTTP(ctx, cancel)
	} else {
		SyncStartWithShellTimeOut(ctx, cancel, logInterval, logTimeOut)
	}

	select {
	case <-ctx.Done():
		os.Exit(0)
		return
	}
}
