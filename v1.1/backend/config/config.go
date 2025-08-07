package config

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

var Conf = new(AppConfig)

type AppConfig struct {
	Name      string `mapstructure:"name"`
	Mode      string `mapstructure:"mode"`
	Version   string `mapstructure:"version"`
	StartTime string `mapstructure:"start_time"`
	MachineID int64  `mapstructure:"machine_id"`
	Port      int    `mapstructure:"port"`

	*LogConfig         `mapstructure:"log"`
	*MySQLConfig       `mapstructure:"mysql"`
	*RedisConfig       `mapstructure:"redis"`
	*Tcos              `mapstructure:"tcos"`
	*AliYunAi          `mapstructure:"old_aliyunai"` // 保持小写
	*RabbitMQConfig    `mapstructure:"rabbitmq"`
	*SiliconflowConfig `mapstructure:"siliconflow"`
}

type MySQLConfig struct {
	Host         string `mapstructure:"host"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	DB           string `mapstructure:"dbname"`
	Port         int    `mapstructure:"port"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

type RedisConfig struct {
	Host         string `mapstructure:"host"`
	Password     string `mapstructure:"password"`
	Port         int    `mapstructure:"port"`
	DB           int    `mapstructure:"db"`
	PoolSize     int    `mapstructure:"pool_size"`
	MinIdleConns int    `mapstructure:"min_idle_conns"`
}

type RabbitMQConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	UserName string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}
type LogConfig struct {
	Level      string `mapstructure:"level"`
	Filename   string `mapstructure:"filename"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxAge     int    `mapstructure:"max_age"`
	MaxBackups int    `mapstructure:"max_backups"`
}
type SiliconflowConfig struct {
	APIkey string `mapstructure:"apikey"`
}
type Tcos struct {
	BucketName string `mapstructure:"bucketName"` // 驼峰命名
	Region     string `mapstructure:"region"`
	Host       string `mapstructure:"host"`
	SecretID   string `mapstructure:"secret_id"`
	SecretKey  string `mapstructure:"secret_key"`
	AppID      string `mapstructure:"app_id"` // 下划线命名
}

type AliYunAi struct {
	ApiKey string `mapstructure:"apiKey"`
}

func Init() (err error) {
	viper.SetConfigFile("config/config.yaml") // 使用相对路径
	viper.AutomaticEnv()

	if err = viper.ReadInConfig(); err != nil {
		return fmt.Errorf("viper.ReadInConfig failed: %w", err)
	}

	// 移除配置项打印，避免泄露敏感信息
	// fmt.Printf("Using config file: %s\n", viper.ConfigFileUsed())
	// for _, key := range viper.AllKeys() {
	//     fmt.Printf("%s = %v\n", key, viper.Get(key))
	// }

	if err := viper.Unmarshal(Conf); err != nil {
		return fmt.Errorf("viper.Unmarshal failed: %w", err)
	}

	viper.WatchConfig()
	viper.OnConfigChange(func(in fsnotify.Event) {
		fmt.Println("配置文件修改了:", in.Name)
		if err := viper.Unmarshal(Conf); err != nil {
			fmt.Printf("viper.Unmarshal failed: %v\n", err)
		} else {
			fmt.Println("配置热重载成功")
		}
	})
	return nil
}

func LoadConfig() *AppConfig {
	return Conf
}
