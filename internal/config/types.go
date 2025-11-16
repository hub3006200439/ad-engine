package config

// Config — корневая структура конфига.
type Config struct {
	Server ServerConfig    `toml:"server"`
	PG     PGStorageConfig `toml:"pgstorage"`
	Logger LoggerConfig    `toml:"logger"`
}

// ServerConfig — общие настройки HTTP-сервера.
type ServerConfig struct {
	Host   string             `toml:"host"`
	Port   int                `toml:"port"`
	Name   string             `toml:"name"`
	Params ServerParamsConfig `toml:"params"`
}

// ServerParamsConfig — "производственные" параметры fasthttp.
type ServerParamsConfig struct {
	MaxProcs                 int  `toml:"max_procs"`
	ReadTimeout              int  `toml:"read_timeout"`
	WriteTimeout             int  `toml:"write_timeout"`
	IdleTimeout              int  `toml:"idle_timeout"`
	KeepAlive                bool `toml:"keep_alive"`
	ReadBufferSize           int  `toml:"read_buffer_size"`
	WriteBufferSize          int  `toml:"write_buffer_size"`
	MaxRequestBodySize       int  `toml:"max_request_body_size"`
	DisablePreParseMultipart bool `toml:"disable_preparse_multipart_form"`
	NoDefaultServerHeader    bool `toml:"no_default_server_header"`
	NoDefaultDate            bool `toml:"no_default_date"`
	NoDefaultContentType     bool `toml:"no_default_content_type"`
	CloseOnShutdown          bool `toml:"close_on_shutdown"`
}

// PGStorageConfig — настройки подключения к Postgres.
type PGStorageConfig struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	DBName   string `toml:"db_name"`
	User     string `toml:"user"`
	Password string `toml:"password"`
}

type LoggerConfig struct {
	Level            string `toml:"level"`
	EnableFile       bool   `toml:"enable_file"`
	FilePath         string `toml:"file_path"`
	MaxFileSize      int64  `toml:"max_file_size"`
	MaxBackups       int    `toml:"max_backups"`
	Compress         bool   `toml:"compress"`
	SyncMode         bool   `toml:"sync_mode"`
	BufferSize       int    `toml:"buffer_size"`
	BatchSize        int    `toml:"batch_size"`
	FlushIntervalSec int    `toml:"flush_interval_sec"`
	EnableConsole    bool   `toml:"enable_console"`
}
