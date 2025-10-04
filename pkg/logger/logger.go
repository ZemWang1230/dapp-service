package logger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/gorm"

	errorLogRepo "timelocker-backend/internal/repository/error"
	"timelocker-backend/internal/types"
)

var (
	// 全局日志实例
	globalLogger *zap.Logger
	// 全局开关
	LogEnabled = true
	// Debug模式开关
	DebugEnabled = true
	// 初始化锁
	once sync.Once

	// 数据库记录相关
	globalErrorRepo errorLogRepo.ErrorLogRepository
	dbLogEnabled    = false
	dbLogMutex      sync.RWMutex
)

// LogLevel 日志级别
type LogLevel string

const (
	DEBUG LogLevel = "DEBUG"
	INFO  LogLevel = "INFO"
	WARN  LogLevel = "WARN"
	ERROR LogLevel = "ERROR"
	FATAL LogLevel = "FATAL"
)

// Config 日志配置
type Config struct {
	Level         LogLevel `json:"level"`
	EnableConsole bool     `json:"enable_console"`
	EnableFile    bool     `json:"enable_file"`
	FilePath      string   `json:"file_path"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Level:         DEBUG,
		EnableConsole: true,
		EnableFile:    false,
		FilePath:      "./logs/timelocker.log",
	}
}

// Init 初始化日志系统
func Init(config *Config) {
	once.Do(func() {
		if config == nil {
			config = DefaultConfig()
		}

		// 创建编码器配置
		encoderConfig := zapcore.EncoderConfig{
			TimeKey:        "time",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalColorLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}

		// 创建控制台编码器
		consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

		// 设置日志级别
		var level zapcore.Level
		switch config.Level {
		case DEBUG:
			level = zapcore.DebugLevel
		case INFO:
			level = zapcore.InfoLevel
		case WARN:
			level = zapcore.WarnLevel
		case ERROR:
			level = zapcore.ErrorLevel
		case FATAL:
			level = zapcore.FatalLevel
		default:
			level = zapcore.InfoLevel
		}

		// 创建写入器
		var cores []zapcore.Core

		// 控制台输出
		if config.EnableConsole {
			consoleCore := zapcore.NewCore(
				consoleEncoder,
				zapcore.AddSync(os.Stdout),
				level,
			)
			cores = append(cores, consoleCore)
		}

		// 文件输出
		if config.EnableFile {
			// 确保日志目录存在
			if err := os.MkdirAll(filepath.Dir(config.FilePath), 0755); err != nil {
				fmt.Printf("Failed to create log directory: %v\n", err)
			} else {
				file, err := os.OpenFile(config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
				if err != nil {
					fmt.Printf("Failed to open log file: %v\n", err)
				} else {
					// 文件输出使用无颜色的编码器
					fileEncoderConfig := encoderConfig
					fileEncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder // 无颜色
					fileCore := zapcore.NewCore(
						zapcore.NewJSONEncoder(fileEncoderConfig),
						zapcore.AddSync(file),
						level,
					)
					cores = append(cores, fileCore)
				}
			}
		}

		// 创建logger
		core := zapcore.NewTee(cores...)
		globalLogger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	})
}

// getCaller 获取调用者信息
func getCaller(skip int) string {
	_, file, line, ok := runtime.Caller(skip + 2)
	if !ok {
		return "unknown"
	}

	// 只保留文件名，不要完整路径
	filename := filepath.Base(file)
	return fmt.Sprintf("%s:%d", filename, line)
}

// getFunction 获取调用函数名
func getFunction(skip int) string {
	pc, _, _, ok := runtime.Caller(skip + 2)
	if !ok {
		return "unknown"
	}

	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknown"
	}

	name := fn.Name()
	// 只保留函数名，去掉包路径
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		name = name[idx+1:]
	}
	if idx := strings.LastIndex(name, "."); idx != -1 {
		name = name[idx+1:]
	}

	return name
}

// 确保logger已初始化
func ensureLogger() {
	if globalLogger == nil {
		Init(DefaultConfig())
	}
}

// ReInit 重新初始化logger（用于配置更改）
func ReInit(config *Config) {
	if config == nil {
		config = DefaultConfig()
	}

	// 创建编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 创建控制台编码器
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

	// 设置日志级别
	var level zapcore.Level
	switch config.Level {
	case DEBUG:
		level = zapcore.DebugLevel
	case INFO:
		level = zapcore.InfoLevel
	case WARN:
		level = zapcore.WarnLevel
	case ERROR:
		level = zapcore.ErrorLevel
	case FATAL:
		level = zapcore.FatalLevel
	default:
		level = zapcore.InfoLevel
	}

	// 创建写入器
	var cores []zapcore.Core

	// 控制台输出
	if config.EnableConsole {
		consoleCore := zapcore.NewCore(
			consoleEncoder,
			zapcore.AddSync(os.Stdout),
			level,
		)
		cores = append(cores, consoleCore)
	}

	// 文件输出
	if config.EnableFile {
		// 确保日志目录存在
		if err := os.MkdirAll(filepath.Dir(config.FilePath), 0755); err != nil {
			fmt.Printf("Failed to create log directory: %v\n", err)
		} else {
			file, err := os.OpenFile(config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				fmt.Printf("Failed to open log file: %v\n", err)
			} else {
				// 文件输出使用无颜色的编码器
				fileEncoderConfig := encoderConfig
				fileEncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder // 无颜色
				fileCore := zapcore.NewCore(
					zapcore.NewJSONEncoder(fileEncoderConfig),
					zapcore.AddSync(file),
					level,
				)
				cores = append(cores, fileCore)
			}
		}
	}

	// 创建logger
	core := zapcore.NewTee(cores...)

	// 关闭旧的logger
	if globalLogger != nil {
		globalLogger.Sync()
	}

	globalLogger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
}

// InitDB 初始化数据库记录功能
func InitDB(db *gorm.DB) {
	dbLogMutex.Lock()
	defer dbLogMutex.Unlock()

	if db != nil {
		globalErrorRepo = errorLogRepo.NewErrorLogRepository(db)
		dbLogEnabled = true
	}
}

// SetDBLogEnabled 设置数据库记录是否启用
func SetDBLogEnabled(enabled bool) {
	dbLogMutex.Lock()
	defer dbLogMutex.Unlock()
	dbLogEnabled = enabled
}

// IsDBLogEnabled 检查数据库记录是否启用
func IsDBLogEnabled() bool {
	dbLogMutex.RLock()
	defer dbLogMutex.RUnlock()
	return dbLogEnabled && globalErrorRepo != nil
}

// recordErrorToDB 记录错误到数据库
func recordErrorToDB(errorType, msg string, err error, fields ...interface{}) {
	if !IsDBLogEnabled() {
		return
	}

	// 构建上下文信息
	contextMap := make(map[string]interface{})

	// 添加调用者信息
	caller := getCaller(1)
	function := getFunction(1)
	contextMap["caller"] = caller
	contextMap["function"] = function

	// 添加错误信息
	if err != nil {
		contextMap["error"] = err.Error()
	}

	// 添加额外字段
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			key := fmt.Sprintf("%v", fields[i])
			value := fields[i+1]
			contextMap[key] = value
		}
	}

	// 创建错误日志记录
	errorLog := &types.ErrorLog{
		ErrorType:    errorType,
		ErrorMessage: msg,
		Context:      contextMap,
	}

	// 异步写入数据库，避免阻塞主流程
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		dbLogMutex.RLock()
		repo := globalErrorRepo
		dbLogMutex.RUnlock()

		if repo != nil {
			if err := repo.CreateErrorLog(ctx, errorLog); err != nil {
				// 如果数据库写入失败，记录到标准日志（但不再次写入数据库，避免无限循环）
				ensureLogger()
				globalLogger.Error("Failed to write error log to database",
					zap.Error(err),
					zap.String("original_msg", msg),
					zap.String("error_type", errorType))
			}
		}
	}()
}

// BatchRecordErrors 批量记录错误到数据库
func BatchRecordErrors(errorLogs []types.ErrorLog) error {
	if !IsDBLogEnabled() {
		return fmt.Errorf("database logging not enabled")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dbLogMutex.RLock()
	repo := globalErrorRepo
	dbLogMutex.RUnlock()

	if repo == nil {
		return fmt.Errorf("error log repository not initialized")
	}

	return repo.BatchCreateErrorLogs(ctx, errorLogs)
}

// Debug 调试日志
func Debug(msg string, fields ...interface{}) {
	if !LogEnabled || !DebugEnabled {
		return
	}
	ensureLogger()

	caller := getCaller(0)
	function := getFunction(0)

	zapFields := []zap.Field{
		zap.String("caller", caller),
		zap.String("function", function),
	}

	// 处理额外字段
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			key := fmt.Sprintf("%v", fields[i])
			value := fields[i+1]
			zapFields = append(zapFields, zap.Any(key, value))
		}
	}

	globalLogger.Debug(msg, zapFields...)
}

// Info 信息日志
func Info(msg string, fields ...interface{}) {
	if !LogEnabled {
		return
	}
	ensureLogger()

	caller := getCaller(0)
	function := getFunction(0)

	zapFields := []zap.Field{
		zap.String("caller", caller),
		zap.String("function", function),
	}

	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			key := fmt.Sprintf("%v", fields[i])
			value := fields[i+1]
			zapFields = append(zapFields, zap.Any(key, value))
		}
	}

	globalLogger.Info(msg, zapFields...)
}

// Warn 警告日志
func Warn(msg string, fields ...interface{}) {
	if !LogEnabled {
		return
	}
	ensureLogger()

	caller := getCaller(0)
	function := getFunction(0)

	zapFields := []zap.Field{
		zap.String("caller", caller),
		zap.String("function", function),
	}

	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			key := fmt.Sprintf("%v", fields[i])
			value := fields[i+1]
			zapFields = append(zapFields, zap.Any(key, value))
		}
	}

	globalLogger.Warn(msg, zapFields...)
}

// Error 错误日志
func Error(msg string, err error, fields ...interface{}) {
	if !LogEnabled {
		return
	}
	ensureLogger()

	caller := getCaller(0)
	function := getFunction(0)

	zapFields := []zap.Field{
		zap.String("caller", caller),
		zap.String("function", function),
	}

	if err != nil {
		zapFields = append(zapFields, zap.Error(err))
	}

	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			key := fmt.Sprintf("%v", fields[i])
			value := fields[i+1]
			zapFields = append(zapFields, zap.Any(key, value))
		}
	}

	globalLogger.Error(msg, zapFields...)

	// 同时记录到数据库
	recordErrorToDB("ERROR", msg, err, fields...)
}

// ErrorWithStack 带堆栈的错误日志
func ErrorWithStack(msg string, err error, fields ...interface{}) {
	if !LogEnabled {
		return
	}
	ensureLogger()

	caller := getCaller(0)
	function := getFunction(0)

	zapFields := []zap.Field{
		zap.String("caller", caller),
		zap.String("function", function),
		zap.Stack("stack"),
	}

	if err != nil {
		zapFields = append(zapFields, zap.Error(err))
	}

	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			key := fmt.Sprintf("%v", fields[i])
			value := fields[i+1]
			zapFields = append(zapFields, zap.Any(key, value))
		}
	}

	globalLogger.Error(msg, zapFields...)
}

// Fatal 致命错误日志
func Fatal(msg string, err error, fields ...interface{}) {
	if !LogEnabled {
		return
	}
	ensureLogger()

	caller := getCaller(0)
	function := getFunction(0)

	zapFields := []zap.Field{
		zap.String("caller", caller),
		zap.String("function", function),
		zap.Stack("stack"),
	}

	if err != nil {
		zapFields = append(zapFields, zap.Error(err))
	}

	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			key := fmt.Sprintf("%v", fields[i])
			value := fields[i+1]
			zapFields = append(zapFields, zap.Any(key, value))
		}
	}

	globalLogger.Fatal(msg, zapFields...)
}

// SetLevel 动态设置日志级别
func SetLevel(level LogLevel) {
	// 这里可以根据需要重新初始化logger
	// 为了简化，暂时通过全局变量控制
	switch level {
	case DEBUG:
		DebugEnabled = true
	default:
		DebugEnabled = false
	}
}

// Enable 启用日志
func Enable() {
	LogEnabled = true
}

// Disable 禁用日志
func Disable() {
	LogEnabled = false
}

// EnableDebug 启用调试日志
func EnableDebug() {
	DebugEnabled = true
}

// DisableDebug 禁用调试日志
func DisableDebug() {
	DebugEnabled = false
}

// Sync 刷新日志缓冲区
func Sync() {
	if globalLogger != nil {
		globalLogger.Sync()
	}
}
