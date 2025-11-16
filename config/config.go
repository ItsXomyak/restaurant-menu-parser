package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Структуры конфига
type Config struct {
	Env    string       `mapstructure:"env"`
	HTTP   HTTPConfig   `mapstructure:"http"`
	Mongo  MongoConfig  `mapstructure:"mongo"`
	Rabbit RabbitConfig `mapstructure:"rabbit"`
	Google GoogleConfig `mapstructure:"google"`
	Logger LoggerConfig `mapstructure:"logger"`
}

type HTTPConfig struct {
	Port string `mapstructure:"port"`
}

type MongoConfig struct {
	URI string `mapstructure:"uri"`
	Name string `mapstructure:"name"`
}

type RabbitConfig struct {
	URI string `mapstructure:"uri"`
}

type GoogleConfig struct {
	ServiceAccountJSON string `mapstructure:"service_account_json"`
}

type LoggerConfig struct {
	Level string `mapstructure:"level"`
}

// New загружает конфигурацию из переменных окружения
func New() (Config, error) {
	var config Config

	// 1. Значения по умолчанию
	viper.SetDefault("http.port", "8080")
	viper.SetDefault("logger.level", "info")
	viper.SetDefault("env", "local")

	// 2. Чтение из ENV
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 3. Явное связывание ключей конфигурации с ENV
	viper.BindEnv("mongo.uri", "MONGO_URI")
	viper.BindEnv("mongo.name", "MONGO_DB_NAME")
	viper.BindEnv("rabbit.uri", "RABBIT_URI")
	viper.BindEnv("google.service_account_json", "GOOGLE_SERVICE_ACCOUNT_JSON")
	viper.BindEnv("env", "ENV")
	viper.BindEnv("logger.level", "LOGGER_LEVEL")
	viper.BindEnv("http.port", "HTTP_PORT")

	// 4. Загружаем конфиг в структуру
	if err := viper.Unmarshal(&config); err != nil {
		return config, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 5. Валидация
	if config.Mongo.URI == "" {
		return config, fmt.Errorf("mongo.uri (или MONGO_URI) не установлен")
	}
	if config.Rabbit.URI == "" {
		return config, fmt.Errorf("rabbit.uri (или RABBIT_URI) не установлен")
	}
	if config.Google.ServiceAccountJSON == "" {
		return config, fmt.Errorf("google.service_account_json (или GOOGLE_SERVICE_ACCOUNT_JSON) не установлен")
	}

	return config, nil
}
